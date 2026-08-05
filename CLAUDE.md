# CLAUDE.md

本文件为 AI 编码代理与新接手的人提供 yyds 框架的整体指引。

**定位**：yyds 是整套 hwcer 游戏服务端技术栈的**装配入口**——单看某个子仓库（updater、
gateway、cosgo）都只能看到一块，本文补两样东西：

1. **它们怎么拼成一个能跑的游戏服**（启动、请求链路、注册、鉴权）；
2. **跨项目会重复踩的坑**——下面每一条都对应真实项目里已经发生过的线上/联调事故，
   其中数据层那几条抓出过可白嫖的经济漏洞。

子模块自身的实现细节由各自的 README / CLAUDE.md 负责，本文只索引，不复述。

## Build & Test

```bash
go build ./...
go test ./...
go vet ./...
```

## 技术栈全景

yyds 不自己造轮子，它把下面这些仓库组装成游戏服。**修 bug 前先判断问题落在哪一层**，
这是最省时间的一步：

| 仓库 | 职责 | 什么问题该去这里找 |
|------|------|-------------------|
| `cosgo` | 应用生命周期、事件总线、配置（viper/toml）、**registry 路由注册** | 模块起不来、接口路径不对 |
| `cosnet` | TCP/WebSocket 传输层，10 字节消息头 | 粘包、断线、心跳 |
| `gateway` | 消息路由分发、连接管理、token/secret/心跳/重连、**接口鉴权** | 协议号找不到路由、鉴权级别不对 |
| `cosrpc` | 内部服务间 RPC（rpcx 封装） | 跨服调用、服务发现 |
| `cosweb` | HTTP 接口（运营回调） | 充值回调、后台接口 |
| `cosmo` | MongoDB ORM（BulkWrite/索引/游标） | 落库、索引、查询 |
| `updater` | 玩家数据更新框架（Updater/Collection/Document） | 道具数量不对、数据没落库、回滚 |
| `logger` | 结构化日志 | — |
| **`yyds`** | **装配层**：Player、Context、配置表、条件验证、事件、排行榜、社交 | 请求生命周期、玩家状态、条件校验 |

## 启动流程

业务层用 `cosgo.Use()` 按序注册模块，`cosgo.Start(true)` 依次调用各模块的 `Init() → Start()`：

```text
cosgo.Start
  ├─ 业务 config.Module.Init()   → 加载配置表
  ├─ yyds.Module.Init()          → options.Initialize、校验 appid/sid、
  │                                 置运营开关、向 master 上报、注册 Metadata
  │     └─ ServerStartHandle     → master 回包交业务层解析（见下）
  ├─ 业务 game.Module.Init()     → 触发业务子包 init()（model/itypes/handle 注册）
  └─ gateway.Module.Init()       → 绑端口、注册 transform 与鉴权规则
cosgo.EventTypLoaded             → players.Start（预加载活跃玩家）
```

**`Module.ServerStartHandle`**：master 在启动上报的回包里下发本服权威运营状态。框架**故意
不解析**这个回包（老版本 master 回 `true`，新版本回状态对象，格式不统一），原样交给业务层。
业务层实现时必须扛两件事，否则会把维护标记抹成"正常开服"：

- `Unmarshal` 失败属正常情况 → 降级沿用配置默认值，**不要让启动失败**；
- 回包可能没有 data，此时 `Unmarshal` 不报错但目标结构全是零值 → 用自己的字段
  （如 `sid != 0`）校验有效性再覆盖。

> ⚠ **不要把 master 下发的参数 merge 进 `options.Game.Values` 这类普通 map**。那套做法已从
> 框架移除：master 推送写与玩家请求读并发访问会直接触发 Go 的 `concurrent map read and
> write` fatal（进程直接挂，不是 panic recover 能兜的）。运营开关走 `players.Maintain()` /
> `players.Creatable()` 这类并发安全接口，其余走 `ServerStartHandle` 回调。

> ⚠ `autoServerId()` 在未显式配 sid 时用 `utils.LocalIPv4()` 的后两段推导服务器编号。
> 多网卡机器（装了 Docker / WSL）上可能选到 `172.x` 虚拟网卡，推出错误的 sid 且不报错。
> **生产环境显式配 `[game].sid`。**

## 请求全链路

```text
[客户端] --cosnet--> [gateway]
     transform: 协议号 code ↔ 路径 path
     authorize: 判定 OAuthType
        ↓ cosrpc
[yyds/context.handlerCaller]        ← 请求生命周期的中枢，读它就懂了一半
     ① 内网 RPC（非客户端路径）→ 直接进 handler，不加载玩家数据
     ② players.Serviceable()   → 未启动/关闭中/维护中一律拒绝
     ③ 按 OAuthType 分级放行
     ④ players.Get(uid) 进玩家协程
        → 状态校验 / 顶号校验（网关不一致即 ErrReplaced）
        → 跨天校验（Login 早于今日零点且非续约路径 → ErrNeedResetSession）
        → 请求重发去重（按 metadata 里的 _rid，命中直接回上次的包）
     ↓
[业务 handler]  func(*context.Context) interface{}
     ↓ 返回后由框架统一收尾（caller）
     Player.Submit() → Dirty.Pull() → Serialize → 回包
```

**收尾是框架做的**：handler 返回后框架自动 `Submit()` 并把数据变更塞进回包。
**业务 handler 不要自己 `Submit()`**（见「提前拿结果」一节）。

## 接口注册（registry）

```go
context.Register(i, prefix...)         // 走网关，客户端可访问
context.RegisterPrivate(i, prefix...)  // 仅内网，客户端访问不到
```

`prefix` 支持三个占位符（`cosgo/registry.Service.format`），**这是控制路由形状的唯一手段**：

| 占位符 | 替换成 | 例（struct `Shop`，方法 `Submit`） |
|--------|--------|-----------------------------------|
| `%v` | 结构体名/方法名（默认值） | `/handle/shop/submit` |
| `%s` | 结构体名 | `/handle/shop` |
| `%m` | 方法名 | `/handle/submit` |

不含占位符的 prefix 按字面量拼接，可直接指定完整路径：
`context.Register(i.ResetStage, "debug/reset/stage")` → `/handle/debug/reset/stage`。

三条经验：

1. **路径即协议**。默认注册（`%v`）下路径由 Go 结构体名 + 方法名拼出来，**重命名结构体
   就会静默改路由**，没有任何编译错误。想让重构与路由解耦，就注册函数并显式给路径，
   或用 `%m` 固定只取方法名。
2. **对象注册务必实现 `Caller`**，否则框架走反射调用。未实现时只在启动日志里
   `logger.Debug` 提示一句，很容易漏掉：

   ```go
   func (this *Shop) Caller(node *registry.Node, c *context.Context) interface{} {
       f := node.Method().(func(*Shop, *context.Context) interface{})
       return f(this, c)
   }
   ```

3. **签名不对的方法会被静默过滤**（`handlerFilter`）：函数须是
   `func(*context.Context) interface{}`，方法须 2 入参 1 返回值。写错签名的表现不是编译
   报错，而是**这个接口根本不存在**。

排查"接口 404"先用 `context.Service.Paths()` / `Range()` 枚举实际注册结果，
能立刻区分「没注册上」和「路径和你以为的不一样」。

## 鉴权（gwcfg.Authorize）

四级，默认 `OAuthTypePlayer`：

| 级别 | 含义 |
|------|------|
| `OAuthTypeNone` | 不需要登录（ping/login） |
| `OAuthTypeOAuth` | 已认证、未选角（roles/create/select） |
| `OAuthTypeSelect` | 需选角但**不进玩家协程**，无法操作玩家数据 |
| `OAuthTypePlayer` | 需选角并进玩家协程（**默认**，绝大多数游戏接口） |

```go
Authorize.Set(servicePath, serviceMethod, lv)     // 精确路径
Authorize.Prefix(servicePath, serviceMethod, lv)  // 前缀匹配，多个命中取最长
Authorize.Default(lv)                             // 改默认级别
Authorize.SetMaster(servicePath, serviceMethod)   // 标记 GM/开发者接口，前缀匹配
```

**`SetMaster` 是前缀匹配**（`IsMaster` 用 `HasPrefix`），GM 接口靠**路径前缀**成组鉴权：
`SetMaster(ServiceTypeGame, "debug")` 之后 `/debug/*` 全是 GM 接口。推论：

- 给 GM 模块加接口，路径第一段必须仍在那个前缀下，**换前缀 = 把 GM 接口对所有玩家放开**；
- 反过来，把普通业务接口的路径起成 `debug/...` 会意外获得 GM 语义。

> `Set`/`SetMaster` 的参数会被 `path.Join` 后小写化（key 形如 `/game/debug`），不必自己拼斜杠。

> ⚠ 维护期不要指望"开发者放行"：网关只在 OAuth（账号登录）阶段往 metadata 里塞
> `ServiceMetadataDeveloper`，`Player` 这一级根本不带，而绝大多数游戏接口都走后者。
> 按 metadata 判开发者对日常请求恒为假。真要留后门，走内网 RPC 或另加显式标记。

---

# 数据层铁律（updater 协作）

`player.Player` **内嵌 `*updater.Updater`**，业务拿到的 `c.Player` 同时是玩家对象和数据更新器。
这份便利带来了下面一整片坑。**这一节是本文最重要的部分**，跨项目一字不差地适用。

## 🔴 事务只覆盖 operator，不覆盖你对结构体字段的直接赋值

updater 的事务（失败回滚 / 成功落库）**只覆盖 operator**（`Add`/`Sub`/`Set`/`Del`）。
你对 proto 结构体字段的**直接赋值**，事务不知道、也回滚不掉——handler 失败时 operator 被撤，
直接改的那份已落在**常驻内存的在线玩家对象**上，回滚不到也没进库 → 内存与库不一致，
**污染该玩家直到重新加载**。

**取指针的入口**（拿到的都是 store 内存，一律先当只读）：
`Document.Any()`、`Collection.Get(id)`、`role.Xxx[k]`、任何业务侧的 `xxxGet(c)`。

### 判据：三问，任意一句命中即必须走 operator

**别只记"不能改内存"，要能判**：

| # | 问题 | 命中即危险，因为 | 典型 |
|---|------|-----------------|------|
| 1 | **新值依赖旧值吗？** | 残留在内存 = 凭空多算一次，**永不自愈** | `+=`、`++`、`-=`、append 到 map/slice |
| 2 | **值里掺了随机 / 客户端输入 / 一次性抽取结果吗？**（重启后重算不出同一个值） | 残留 = 白拿一次本该付费的结果 | 刷新商店时重新加权随机上架 |
| 3 | **它排在 `Sub`/`Add`/`Verify` 之后吗？是这笔交易的收益或凭证吗？** | operator 在提交期才校验，回滚了它不回滚 → **白嫖或倒扣** | 发奖后 `task.Submitted[id]=1`、扣料后 `bless.Times=&Times{}` |

三问全否 → **幂等绝对重置**：残留在内存也是正确态，重启从库读回旧值会在同一守卫下重算出
同一结果 → 自愈。可以直接赋值，但**必须原地注释"为什么安全"**。**拿不准就走 operator。**

⚠ **第 3 问最容易漏**，因为它与值的形态无关：`Submitted[id]=1` 不读旧值、完全可重算，
前两问都不命中——但它是"已领奖"的凭证，回滚后玩家没拿到奖却被记成已领，重复提交判定
还会拦住重试，**永不自愈**。**只要处在扣料/发货链路上，再幂等的赋值也必须走 operator。**

> 📌 不要用「这个字段算不算权威业务态」当判据——限购计数、章节进度都是货真价实的权威态，
> 照样安全。决定后果的只有上面三问。

### 正确写法

```go
cur  := xxxGet(c)      // 只读：判断、算消耗
next := xxxClone(c)    // 要改 → 先深拷贝（proto.Clone/Merge，别用 *ptr 值拷，会触发 copylocks）
c.Player.Sub(...)      // 扣料（可能在提交期失败）
next.Exp += n          // 改副本
xxxSave(c, next)       // 回写；提交期才装进 dataset，失败则原数据不受影响
```

### 更彻底：把副本语义收进取值入口

逐处克隆是"记得写"才对，漏一处就是一个洞；收进入口则是"想错也错不了"。
**给 store 上每个 map 字段配一个返回副本的 `GetXxx`，业务侧不再允许出现 `store.Xxx[k]`**，
并把周期/兜底判定一并收进去：

```go
func (r *Role) GetPurchase(p *Player, id int32, rule []int64) (*pb.ShopShelf, error) {
    if v := r.Purchase[id]; v != nil && (v.Expire <= 0 || v.Expire >= p.Unix()) {
        dst := &pb.ShopShelf{}
        protoMerge(dst, v)                            // 命中 → 深拷贝
        return dst, nil
    }
    expire, err := p.Times.ExpireWithArray(rule...)   // 未有/已过期 → 重置后的新记录
    if err != nil {
        return nil, err
    }
    return &pb.ShopShelf{Value: 0, Expire: expire}, nil
}
```

两个收益叠加：调用方**拿不到 store 指针**，且**拿到的一定是当前周期**（没有"漏判周期后
继续累加上个周期计数"的机会）。

### 同一个错误的各种外衣

| 外衣 | 错在哪 | 正确写法 |
|------|--------|---------|
| `role.Skills[k]=oid` | 直接改 map 内存 | 走子键 Setter：`Set("skills", oid, k)` |
| `sr.Lv++`、`item.Attach[x]=v` | 直接改字段 | `Add(field, delta)`，或克隆改副本后 `Set` |
| `Get(...).Speed(...)` 后不 `Save` | 改完没回写 | 改完必 `Save`；取值入口返副本堵这一手 |
| `Sub(货币)` 之后再直接改字段 | 前半回滚、后半不回滚 → 半拉子状态 | 一切改动走 operator |
| 校验没过时"改完再 `return err`" | 同上 | 错误分支里不要有任何直接赋值 |
| **`GetXxx()` 里 `return store.Map[k]`** | **根因**，上面每种外衣都由它派生 | 入口返深拷贝副本 |

⚠ 包装函数命名当心语义反转的一对：`Wrap(t)` 通常是 `&T{inner:t}`（**改它就是改原对象**），
`Clone(t)` 才是深拷贝。名字只差一个词，用错就是上表第四行。

### 审计检索启发式

在自己代码里找候选点：

1. grep `+=`、`++`、`.Value =`、`[k] =` 出现在 `.Set(`/`.Save(` **之前**、且左值来自
   `Get(`/`.All()`/`.Any()`/`role.` 的地方；
2. **先按第 3 问反向扫最省力**：直接搜 `Sub(`/`Add(`/`Verify.Auto(` 的调用点，看同一函数里
   前后有没有对 store 指针的直接赋值——交易链路上的赋值不管长什么样都得改，
   这样扫既快又不会被"看着很幂等"骗过去；
3. **错误分支尤其查**："校验没过时改完再 `return err`"；
4. ⚠ 就地删 map（`delete(slots.Slots, k)`）**grep `role.X =` 抓不到**，要连着"从 store 取出来的
   局部变量"一起看。

**这类问题单元测试抓不到**（跨请求、只在失败路径暴露）。验证手法：**把材料清零再调接口，
看状态是否被污染**——真实项目里用这招抓到过「材料为 0 仍能启动修行」「无限刷契合度」
两个白嫖漏洞。

## 🔴 本请求内：operator 只是入队，`Val()` 读到的仍是旧值

`Add`/`Sub`/`Delete` **只是把 operator 压进 statement 队列**，要到 `Submit` 的 Parse 阶段
才真正改 dataset。所以在同一个 handler 里：

- **`Sub` 之后 `Val(iid)` 读到的是【扣料前】的数量**。若要基于余额再做发放，必须自己减去
  本次已排队的消耗，否则会把已扣的那份重复算进收益（真实事故：升星消耗的碎片被 1:1
  原样退成另一种道具，等于免费升星，且持有越多白嫖越多）。
- **重复 `Delete` 同一个 key 会排出多条 Del**，提交期不报错，但计数虚高 →
  批量删除前必须**按 key 去重**。
- `Del` 排在 `Sub` 之后是安全的：Parse 期按当时真实余量整对象删除。

一句话：**在同一请求内，dataset 的内容滞后于你已经下达的指令。**

## 🔴 非常驻数据要先 `Select`/`Data` 预加载

`RAMType` 决定字段是否常驻内存：`RAMTypeAlways` 全量常驻，`RAMTypeMaybe` 按需加载，
`RAMTypeNone` 不缓存。**读非常驻字段前必须先 `u.Select(...)` 登记、经 `Data()` 拉取**，
否则 `u.Val()` 取到零值且不报错。

同理，条件验证前要用 `Verify.Target(...)` 登记预读、必要时 `Player.Data()` 拉取，
再 `Verify.Verify(...)`——`Verify.Auto()` 内部含预读登记，所以业务层不必手动 `Data()`。

## 🔴 手动 `Submit()` 与 `u.Dirty` 是配套的一对

`Submit()` 的返回值 `[]*operator.Operator` **就是本次请求要同步给前端的那份数据**，
且返回时会把内部队列清空：

```go
// updater.go
func (u *Updater) Submit() (r []*operator.Operator, err error) {
    ...
    r = u.dirty
    u.dirty = nil    // ← 取走
    return
}
```

框架在 handler 返回后会自己调一次 `Submit()` 把结果塞进回包。所以业务层**手动 `Submit()`
是允许的**（典型场景：接口内还要写自己那张表，须先确认道具发得出去），
但**必须把返回值放回去**，否则框架那次 Submit 拿到的是空的、**推送丢失**：

```go
ops, err := u.Submit()   // 提前落库，确认道具发得出去
if err != nil {
    return err
}
// ... 写自己那张表 ...
u.Dirty(ops...)          // ← 把推送放回去，否则客户端收不到本次变更
```

`u.Dirty(opt...)` 的实现就是 `u.dirty = append(u.dirty, opt...)`——文档写的
**「设置脏数据，手动更新到客户端，不进行任何操作」**，它天生就是为这个场景准备的补推送通道。

### 什么时候**应该**丢弃 Submit 的返回值

"放回去"是默认动作，不是铁律。以下两种情况**故意不放**才对：

- **登录 / 选角这类后面必定全量拉取的路径**：客户端接着就会拉一遍完整数据，
  增量推送没有任何意义，放回去只是多一份冗余。
- **带外通道改数据**（GM 指令、后台运营、调试工具直接改存档）：这类调用**不在任何客户端
  请求的上下文里**，放回 `u.dirty` 的后果是——这批 operator 会挂到该玩家**下一个毫不相干的
  请求**的回包上，客户端于是把变更归因到那个接口（"我调这个接口为什么获得了这些东西？"）。
  **比不推更糟**。带外修改就让客户端下次全量拉取时自然发现。

判据是**这批变更能不能被客户端正确归因**：能（就是当前请求造成的）→ 放回去；
不能（带外/后续会全量覆盖）→ 丢弃，并在原地注释为什么。

⚠ **但不能拿 `u.Dirty` 当写操作用**：它不进 statement、不会被 Parse 应用到 dataset、
最终不落库。要写数据用 `Collection.Insert(op, before...)` / `Document.Set` / `Updater.Add`。
误用的表现极具迷惑性——客户端看到数据变了，下次请求又变回去（真实事故：门票实例建不出来，
每次请求都重建、客户端反复收到 New，体力恢复不持久）。

> `Insert(op, before=true)` 前插用于"本次结算必须排在业务扣量之前"的场景，
> 否则"先扣后回"会拿旧值判定不足。

> ⚠ **两个 Dirty 不是一回事**：`updater.Updater` 内部的 `dirty`（operator 列表，
> Submit 返回、进回包的 `Cache`）与 `player.Player.Dirty`（字段，短连接推送缓存，
> 进回包的 `Dirty`）。且后者**遮蔽**了嵌入的 `Updater.Dirty` 方法——
> 要调 updater 那个必须写 `c.Player.Updater.Dirty(...)`。

## 🔴 dataset model 的 `Init` 每次加载都会跑，必须幂等

`Init` **不是"创角一次性钩子"**：除了建号，每次从库加载该玩家时也会先 `New()` 再
`Find` 覆盖上去，mongo 反序列化是**往 Init 建好的 map 里合并 key、不重建**。
因此 `Init` 必须**幂等无副作用**。

这带来一个常被忽视的红利：**由 `Init` 授予、又不落库的内容，等于纯配置驱动**——
策划改配置全服立刻生效，老号不需要任何数据迁移（真实案例：初始法阵始终不落库，
老号库里没有该字段也能正常使用）。想要这个性质就别在 `Init` 里写 `Set`。

## 提前拿到"这次发放的最终结果"：优先用 `Updater.Verify()`

溢出截断、重复自动分解这类信息在 **Parse 期**才产生，而 Parse 默认发生在 handler 返回
**之后**框架那次 `Submit()` 里，handler 读不到。要在 handler 内读：

- 调 `c.Player.Updater.Verify()`——跑同一个 `data→verify` 循环、overflow 照常触发，
  但**不落库、不清 dirty、不发成功事件**，读完让框架照常收尾即可，没有"忘了把推送放回去"
  的风险（库注释明写「Verify 之后再 Submit 是安全的」：status 已被消耗，
  Submit 的收敛循环直接跳过）；
- 只在**接口内还要单独写库**时才手动 `Submit()`（如领邮件后改邮件状态：先 Verify/Submit
  确认道具发得出去，再写自己那张表，避免"表已改、道具没发"）——此时**务必
  `u.Dirty(ops...)` 把返回值放回去**，见上一节；
- `Verify()` 返回的 error 必须 `return`：Parse 的错误不置 `u.Error`，靠 handler 回非零
  code 才能让框架跳过后续 Submit。吞掉它会落库半成品；
- `Collection.Add(id, value)` 返回 `*operator.Operator`，发放时留住指针，`Verify()` 之后读
  `op.OType` 即可**精确到具体是哪一次发放**（按 iid 事后反查在"同一请求内多次发放同一 iid"
  时分不清是第几次）。

## 🔴 `Player.Verify` / `Player.Dirty` 是字段，遮蔽了嵌入 Updater 的同名方法

`Player` 内嵌 `*updater.Updater` 之后又定义了 `Verify *verify.Verify`（全局条件验证器）
和 `Dirty Dirty`（推送缓存）：

```go
c.Player.Verify()          // 编译失败：*verify.Verify is not a function
c.Player.Updater.Verify()  // 正确
```

给 `Player` 加新字段时要意识到：**与 Updater 方法同名的字段会让所有调用点在编译期炸开**
（好在是编译期）。

## 🔴 `ModelSet.Set` 返回 `ok=false` **不是拒绝**，而是"转反射"

业务 model 实现 `dataset.ModelSet`（`Set(k string, v any) (any, bool)`）来接管字段写入。
很容易以为返回 `false` 等于"这个 key 我不认、别写"，**事实相反**：

```go
// dataset/document.go: setter
if m, ok := doc.data.(ModelSet); ok {
    if r, ok = m.Set(k, v); ok {
        return                       // 业务处理了 → 用业务的结果
    }
}
sch, err := doc.Schema()             // ← ok==false 落到这里
logger.Debug("建议给%v.%v添加Set接口提升性能", sch.Name, k)
return v, sch.SetValue(doc.data, v, k)   // 按字段名反射写入，照样生效
```

所以 `Set` 的 switch **漏写一个 case 不会导致写入失败**，只是从"类型断言直写"降级成
"反射写入"外加一条 Debug 日志。落库同样正常：cosmo 的 `update.Transform` 会按 schema 把
Go 字段名映射成 DBName（`SpiritFormations` → `spiritformations`），不会写出大小写不一致的
野字段；含 `.` 的 key 则原样下发。

**推论——想真正否决一次写入，`return false` 是没用的**，必须在 `Set` 里 panic
或置 `u.Error`。

> 📌 这条是纠正来的：曾据"`setFromHandle` 对不含 `.` 的 key return false"推断
> "只有子键 handle 的 map 字段整字段写入会被静默丢弃"，并据此在 GM 工具里加了一道
> 可达性守卫。**推断错了**——写探针测试实测：整字段 `Set` 之后内存与 dirty 都正确，
> 那道守卫只是拦下了本可成功的写入。**读到 `return false` 别停在这一层，要跟到
> 框架拿它做什么。**

真正会被**静默丢弃**的是另一处——`dataset.Document.Set` 的前置检查：

```go
func (doc *Document) Set(k string, v any) {
    if !doc.Has(k) { return }        // 字段不在 schema 里 → 直接返回
    ...
}
```

`Has` 按 schema 查字段（`a.b` 只查 `a`），查不到打一条 `logger.Alert` 就返回。
同理 `update.Transform` 里 `LookUpField` 找不到的 key 会被悄悄剔出更新语句。
**即写错字段名才是那个"看起来成功、其实没写"的场景**，与子键 handle 无关。

---

## 条件验证（players/verify）

通用条件校验系统，用于任务完成判定、解锁条件检查。配置结构实现 `verify.Target`
（`GetCondition()`/`GetKey()`/`GetGoal()`），默认比较方式 `>=`。

```go
if cfg.GetCondition() > 0 {
    if err := c.Player.Verify.Verify(cfg); err != nil {
        return err   // 条件不满足
    }
}
c.Player.Verify.Auto(cfg)  // 自动版：内部含预读登记，失败时置 u.Error 让整个请求回退
```

框架内置条件类型（`players/verify/condition.go`）：

| 值 | 常量 | 取值方式 |
|----|------|---------|
| 0 | `ConditionNone` | 无条件，直接通过 |
| 1 | `ConditionData` | 基础数据 / 日常 / 成就记录 |
| 2 | `ConditionEvents` | 即时监听事件（仅任务系统） |
| 9 | `ConditionMethod` | 调用注册的 Method 函数 |
| 101 | `ConditionWeekly` | 周数据（基于 daily） |
| 102 | `ConditionHistory` | 历史数据 |

业务可用 `verify.Register(key, handle)` 注册自己的条件类型，**也可以覆盖内置的**
（项目里常把 `ConditionData` 重注册成查自己的角色字段）。

## 事件监听（players/emitter）

```go
p.Listen(name string, key int32, args []int32, handle emitter.Listener) (*emitter.Context, error)
p.On(key int32, args []int32, handle emitter.Callback) *emitter.Context
p.Emit(key int32, val int32, args ...int32)
```

- **`name` 同名自动去重**（覆盖旧监听），业务层不必自己维护去重状态——这是 `Listen` 相对
  `On` 的主要价值。命名约定用「模块名_业务id」（`break_1`、`task_233`），同模块多个并行
  监听才不会互相覆盖。
- 回调返回 `false` 表示移除监听。
- 上下文通过 `Context.Attach` 传递（注册时 `l.Attach.Set("id", taskId)`，回调里
  `att.GetInt32("id")` 取回），不要用闭包捕获——监听会跨请求存活。

## 玩家协程与并发

- **handler 已经跑在该玩家的协程/锁里**，再对**自己**调一次 `players.Get(c.Uid())`，
  Actor 模式下直接卡死 channel。手上现成的 `c.Player` 直接用。
- `Player.Send` 要求调用方持有玩家锁。从 daemon、定时器或**别的**玩家协程里推消息，
  必须套 `players.Get(uid, func(p){ p.Send(...) })`。
- 跨玩家操作（交易、好友）用 `players.Locker(self, uids, ...)` 同时锁定多个玩家，
  不要嵌套 `players.Get`。
- `p.Status` 是**无锁 CAS 状态机**（disconnect/offline/recycling 的 CAS 都在玩家锁之外），
  持有玩家锁也不能裸读；要读就 `atomic.LoadInt32` 一次存局部变量——两次读之间 daemon
  可能已经翻了状态。

## 早退分支必须走 `Serialize`

在 `handlerCaller` 层拒绝请求（维护中、顶号、跨天）时，两种想当然的写法都不对：

```go
return nil, err  // 错误落进 RPC 的 error 槽，被当系统级错误，客户端收不到业务码
return err, nil  // 裸 error 没经过 Serialize，客户端按协议解只能得到默认错误码
```

必须 `return Serialize(c, Error(err))`（框架内即 `c.reject(err)`）。实测：维护拦截用第二种
写法时客户端收到的是默认码。

**handler 返回 `[]byte` 会跳过 Message 封装**：框架只做 `Submit()` 后原样透传，不再封装
code/time/dirty。需要自定义协议体时才这么用。

## 接入约定

- **`go.mod` 不用 `replace` 指向本地路径**，一律按伪版本号从 GitHub 拉。改框架源码不会对
  业务服生效，必须走：改框架 → `git push` → 业务侧 `go get github.com/hwcer/<pkg>@main`
  → 提交 `go.mod`/`go.sum`。调试期临时加 `replace` 可以，**提交前必须移除**，
  否则别人机器和 CI 上没有那个路径，直接构建失败。
- **版本推进顺序**：底层（updater）先发 → 上层（yyds）bump → 业务服 bump。
  一次功能改动可能要动两个仓库并各自发版。

## 调试

`gomcp/` 是内嵌进游戏服进程的 MCP 调试服务：进程状态、在线玩家、路由列表、
按玩家身份调任意 handle、pprof 采样。配了 `[mcp].address` 才启用。

它是排查上面那些坑的主力工具——**「把材料清零 → 调接口 → 看状态是否被污染」这套验证
无法用单元测试完成**，只能对着真实进程打真协议。业务侧可以往里注册自己的工具
（配置表查询、GM 指令、存档读写等依赖业务数据结构的，yyds 作为通用框架不该认识它们）。

> 写这类"直接改存档"的调试工具时注意：它绕过所有业务校验，也**绕过业务的配套动作**。
> 例如"解锁某功能"在业务侧是「写解锁标记 + 删除进行中的计时」两步，工具只做前一步就会
> 留下永远走不完的倒计时。**能做成 GM 接口就别做成裸改存档**——GM 接口可以复用业务逻辑。

## 子模块文档索引

本文不复述这些，遇到对应问题直接去读：

| 文档 | 内容 |
|------|------|
| `README.md` | 模块清单、并发模式（Locker / Actor）选择 |
| `players/README.md` | 玩家生命周期、7 个状态迁移、事件、内存回收 |
| `options/README.md` | 运行时配置、`Setting` 可插拔函数（GetIType/GetIMax/Renewal） |
| `modules/{rank,graph,chat,locator}/README.md` | 排行榜 / 社交图谱 / 聊天 / 全服定位 |
| `updater` 仓库的 `CLAUDE.md` | **updater 内部实现**：四种数据模型、IType 路由、RAMType、Dirty 追踪、operator 流水线、熔断。改数据层前必读 |

## Language

代码注释、错误信息、文档一律中文，保持此约定。
