# MOUNT — 挂载数据层机制深潜（实测溯源）

> 本文是 CLAUDE.md「要在同一 handler 里写另一张表：优先 Mount」小节的深化专题：
> 基础四条（operator 流水线 / Receive≠Insert / 卸载粒度 / schema 声明）见 CLAUDE.md，不在此重复。
> 全部结论来自 2026-09 ProjectElf「探索轻量战斗 + stage 会话迁 Mount」实战溯源，
> 经 dev21 真协议 + Mongo 探针验证（updater v1.5.1+）。实战案例索引见文末。

## 一、operator 去向：下发与否没有开关字段

挂载 op 是否随 `S2CUpdate` 下发，**由模型是否声明 `ModelIType` 决定，没有独立开关**：

| 模型形态 | operator 去向 |
|---|---|
| 实现 `ModelIType` 且 `IType(0)` 返回非 0 | 进 `Updater.dirty` → 随 S2CUpdate 下发客户端 |
| 未声明（itype=0） | `newMount` 挂 `mountDiscard` 接收器，op 直接丢弃 |

- 🔴 **判据是「`IType(0)` 返回非 0」，不是「实现了 ModelIType」**——项目侧模型基类往往自带
  `IType(iid)` 转发全局配置，于是每个模型都"自动满足"接口；想让挂载走通用更新，
  必须显式覆盖 `IType` 返回自己的类型，光加接口断言不起作用。
- 纯服务端数据（会话态、冷数据、战斗中状态）**不声明 IType** 是推荐形态：
  客户端对这条数据无感知，下行走业务自己的显式协议。
- `Mount.Operators()`：读 `statement.cache`，**时机很具体**——op 在 verify 阶段进 cache
  （业务手动 `u.Verify()` 之后可读），submit 阶段 cache 交给接收器并置空，**那之后再读是空的**。
  「模型没声明 IType、要自己组装协议告诉客户端」的挂载，全靠这条路知情。

## 二、五件套契约与 Upsert 语义

`MountModel` 五件套：`TableName` / `Schema` / `Upsert` / `Getter(u, coll, keys)` / `Setter(u, bw, _id, dirty, unset)`。

- 🔴 **`Upsert` 不会被走到**：Mount 的 parseSet 对未知 OID 直接报错，新行必须显式 `Insert`；
  保留该接口只是签名要求。返回 `true` 与 Collection 侧「Set 未知键自动建」对齐，留作后手。
- **op 的内存应用路由到模型自身的 `Get/Set(k, v)`**（dataset.Document 经 schema/接口分发）：
  dotted 子键（`"grid.5"`）支持与否**看模型自己的实现**，框架不代做——模型的 `Set` 里写好
  `strings.Split(k, ".")` 分发才有。整字段写（map 整存）永远可用，是省心路径。
- 🔴 **要写的字段必须在模型 schema/`Set` 里声明**：查不到字段名直接丢（只打一行 Alert），
  「直写改挂载后静静少写一个字段」的坑见 CLAUDE.md 基础四条。

## 三、写路径三坑（全部静默或半静默）

1. **`Update`/`Set` 对未知 OID：parseSet 直接报错**——新行必须显式 `Insert`，
   「先 Update 不存在就当 Insert」的写法不存在。
2. **`Insert` 要求对象自带 `_id`**（`dataset.NewDoc` 取 `Fields.OID`）：业务侧构造行对象时
   必须先填主键。装备类不可堆叠道具的 oid 生成走项目侧 `IType.GetOID`（内部 `ObjectId.New`），
   **可堆叠与不可堆叠的生成规则不同**，别自己拼字符串。
3. **同 OID 先 `Delete` 再 `Insert`：BulkWrite 是「Delete → 非 upsert Update」序 → 静默 0 行**。
   覆盖已存在的行用**整字段 `Update`**（`coll.Update(id, dataset.Update{全字段})`），
   不要删了重插。

## 四、与玩家数据同滚：Mount vs 直写的选型判据

| | Mount op | cosmo 直写（`db.Model(...).Update`） |
|---|---|---|
| 生效时点 | 提交期（请求末尾统一 Submit） | **立即** |
| 请求失败 | 随 operator 整体回滚 | **已写出的不回滚** |
| 典型事故 | — | 「标记已领、东西没发」；「忘调落库=静默丢数据」（漏调直写接口，重启后状态回滚） |

- 会话态 / 中间状态 / 与玩家数据有原子性要求的数据：**一律 Mount**。
- 选型零扰动迁移法：当某张表的数据出口收敛在一个文件（接口层）时，
  **换内核保签名**——接口签名与调用方不动，实现从直写换成 mount op
  （直写接口的 `error` 返回可保留但恒返 nil，失败经 `Updater.Error` 提交期统一爆；
  调用方的 `if err :=` 判错不用改）。

## 五、直插（`Collection.New`）vs 工厂（`Add`）：预生成数据的唯一通道

两条入包路径（`parse_coll.go` / `handle_coll.go`）：

```text
Add(iid, val) → collectionHandleAdd → 不可堆叠 → collectionHandleNewEquip
                 → it.New（业务 creator：容量校验 + 词条生成 + 事件）→ insert（生成 OID）

Collection.New(v) → TypesNew op（op.Result=[v]）→ collectionHandleNew 原样落库
```

- 🔴 **creator 拿到的是框架新建的空 item**（`ITypeCollection.New` 内部构造），
  业务侧**无法预塞 Attach**——「先生成词条、战斗后按同一份入包」这类预生成需求
  只能走 `Collection.New` 直插。
- 🔴 **`Collection.New` 要求 `v.GetOID()` 非空**（`collectionHandleNew` 校验 OID 非空 + `Result`）；
  op.Result ≠ nil 时 `it.New` 被整段跳过（`collectionHandleNewEquip` 的机制）。
- 直插**跳过了 creator**：容量校验（CapacityVerify）与获得事件（Emit）由业务自担，
  漏掉就是「绕过背包上限」。
- 附：**`Attach.Unmarshal` 的目标类型必须匹配写入形态**——写入 `[]struct`（值切片）
  就不能 `Unmarshal` 进 `[]*struct`（cannot assign，且报错点离写入点很远）；
  值切片/指针切片在 updater/values 的类型体系里是两个不相交的动态类型。

## 六、实战案例

| 案例 | 溯源点 |
|---|---|
| ProjectElf `StageLiteBattle`（进战会话挂载，explore lite-battle session） | ModelIType 开关 / Operators() 组包 / 直插预生成词条 |
| ProjectElf `StageMount`（stage 会话从 Cache+直写整体迁 Mount，battle 会话内嵌为主字段） | 选型判据 / 零扰动迁移法 / 同 OID 删+插坑 |
| 回归手法：Mongo 探针直读落库态 | 「提交期落库」的验证不能靠内存读——重启/回收才会暴露 |

## 附：cosmo 错误 API 现状（v1.4.2+，2026-09-21）

- `Errorf(nil)` 判 nil 跳过、**不再生成 `values.Message(nil)`**——早期「成功路径 `db.Error`
  装 typed-nil、`return tx.Error` 假报错」的隐患从源头消除，下游不再需要强制 `Err()` 防护。
- 驱动层错误已由 `NormalizeError` 统一转换（可直接读 Code / Args）；`Errorf` 仍是
  `Error` 字段的唯一写入口。
