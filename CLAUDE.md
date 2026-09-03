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

> ⚠ **业务写的启动期自检，在 debug 模式下只告警、不拦启动**（`config.Reload` 的行为）。
> 所以"配置与代码对不上"这类自检**在开发机上永远只是一行日志**，正式模式才拒绝启动。
> 写自检时要按"开发期没人会看日志"来设计：错误信息里直接写清**该跑哪个脚本/改哪张表**，
> 而不是只报"校验失败"。

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
     Player.Submit() → Pending.Pull() → Serialize → 回包
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

### `OAuthTypeSelect` 的真实用途：避开自死锁

它看起来只是"省一把锁"，实际是**一整类服务的唯一可行级别**：某个服的 handler 处理到一半会
回头 RPC 调另一个服（拉玩家档案、落库、结算），而那边一律 `players.Get(uid)` 进**同一个**
玩家锁。同进程共用一份 `players` 管理器时，入口若已持锁，嵌套的 `players.Get` 就永远等不到——
那把锁是 `sync.Mutex`、**不可重入且没有超时**。

所以「自己不碰玩家数据、所有落库交给别的服」的服务，**整个服降到 `OAuthTypeSelect`**，
包内不许出现 `c.Player`。
踩过：一个接口卡满 10s RPC 超时后，该玩家**所有**请求（含心跳、重登）全部跟着超时。

> ⚠ 已移除 actor 模式（`refactor(players)!`）之后这条**没变**——"持锁期间不能对自己再 Get"
> 与用哪种调度实现无关。

### 🔴 `OAuthTypeSelect` 不刷心跳：长时间停在这一级的玩家会被判掉线

`handlerCaller` **只在 `OAuthTypePlayer` 分支调 `p.KeepAlive()`**，而
`players.Options.ConnectedTime`（默认 120s）到点就判断线。于是：玩家整段时间只发 Select 级请求
（客户端心跳又被网关 `transform.C2SHeartbeat` 就地回掉、根本不进玩家协程）→
**整 120 秒后连接必掉**，症状是客户端满屏"超时未收到响应"，分钟数分毫不差。

**这是框架的已知缺陷**（Select 同样代表玩家正在活动，心跳就该刷）。在框架修掉之前，
业务侧只能在那个服的 `Caller` 里每个请求自己调一次 keepalive 顶着；框架修了要把那段删掉。

> 判断自己是否会踩：**这个服有没有可能连续 `ConnectedTime` 秒只收到 Select 级请求**。
> 会 → 现在就得自己刷。

---

# 数据层铁律（updater 协作）

`player.Player` **内嵌 `*updater.Updater`**，业务拿到的 `c.Player` 同时是玩家对象和数据更新器。
这份便利带来了下面一整片坑。**这一节是本文最重要的部分**，跨项目一字不差地适用。

## 🔴 事务只覆盖 operator，不覆盖你对结构体字段的直接赋值

updater 的事务（失败回滚 / 成功落库）**只覆盖 operator**（`Add`/`Sub`/`Set`/`Del`）。
你对数据结构体字段的**直接赋值**，事务不知道、也回滚不掉——handler 失败时 operator 被撤，
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
next := xxxClone(c)    // 要改 → 先深拷贝（别用 *ptr 值拷：结构体带锁时会触发 copylocks）
c.Player.Sub(...)      // 扣料（可能在提交期失败）
next.Exp += n          // 改副本
xxxSave(c, next)       // 回写；提交期才装进 dataset，失败则原数据不受影响
```

### 更彻底：把副本语义收进取值入口

逐处克隆是"记得写"才对，漏一处就是一个洞；收进入口则是"想错也错不了"。
**给 store 上每个 map 字段配一个返回副本的 `GetXxx`，业务侧不再允许出现 `store.Xxx[k]`**，
并把周期/兜底判定一并收进去：

```go
// Shelf 是存档里的一个 map 子对象（值为指针）；调用方永远拿不到 store 里的那一份
func (r *Role) GetShelf(p *Player, id int32, rule []int64) (*Shelf, error) {
    if v := r.Shelves[id]; v != nil && (v.Expire <= 0 || v.Expire >= p.Unix()) {
        return cloneShelf(v), nil                    // 命中 → 深拷贝
    }
    expire, err := p.Times.ExpireWithArray(rule...)   // 未有/已过期 → 重置后的新记录
    if err != nil {
        return nil, err
    }
    return &Shelf{Value: 0, Expire: expire}, nil
}
```

两个收益叠加：调用方**拿不到 store 指针**，且**拿到的一定是当前周期**（没有"漏判周期后
继续累加上个周期计数"的机会）。

**要不要配 `GetXxx`，看 map 的值类型**（别一刀切，全配是过度设计）：

| 值类型 | `store.M[k]` 拿到什么 | 结论 |
|---|---|---|
| **指针**（`map[int32]*Progress`） | store 里那一份的地址，改它就是改常驻内存 | **必须**配返回副本的 `GetXxx` |
| **标量**（`map[int32]int64`、`map[int32]string`） | 值拷贝，改它碰不到 store | **不用**配（写回照旧走 `Set`/`Add`） |

深拷贝用数据结构自带的克隆/合并能力（用 protobuf 的项目即 `proto.Merge`），**别逐字段拷**
——结构体加字段时克隆自动跟上，不会漏。⚠ 当心语义反转的一对：`Wrap(t)` 通常是 `&T{inner:t}`（**改它就是改原对象**），
`Clone(t)` 才是深拷贝。

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
2. **先按第 3 问反向扫最省力**：直接搜 `Sub(`/`Add(`/`Condition.Auto(` 的调用点，看同一函数里
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

同理，条件验证前要用 `Condition.Target(...)` 登记预读、必要时 `Player.Data()` 拉取，
再 `Condition.Verify(...)`——`Condition.Auto()` 内部含预读登记，所以业务层不必手动 `Data()`。

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

> ⚠ **两条 operator 通道不是一回事**：`updater.Updater` 内部的 `dirty`（Submit 返回、
> 进回包的 `Cache`）与 `player.Player.Pending`（跨玩家场景让出锁前 Submit 出来、
> 暂存待发，进回包的 `Dirty` 字段）。
>
> `Pending` 早先叫 `Dirty`，与 `Updater.Dirty` 方法重名造成遮蔽，已改名。

## 🔴 dataset model 的 `Init` 每次加载都会跑，必须幂等

`Init` **不是"创角一次性钩子"**：除了建号，每次从库加载该玩家时也会先 `New()` 再
`Find` 覆盖上去，mongo 反序列化是**往 Init 建好的 map 里合并 key、不重建**。
因此 `Init` 必须**幂等无副作用**。

这带来一个常被忽视的红利：**由 `Init` 授予、又不落库的内容，等于纯配置驱动**——
策划改配置全服立刻生效，老号不需要任何数据迁移（真实案例：初始法阵始终不落库，
老号库里没有该字段也能正常使用）。想要这个性质就别在 `Init` 里写 `Set`。

## 🔴 要在同一 handler 里写"另一张表"：优先 `Mount`，不要直连 DB

典型场景：发完奖还要标记"已领取"、核销一笔订单、占用一个兑换码。直连 DB 写那张表
（绕过 Updater）会留下一个**必然发生的**不一致窗口：直写**立即生效**，而 Updater 要到
handler 返回后才 `Data→Verify→Submit`；一旦提交期失败回滚（容量满、余额不足、creator 报错），
就成了**"标记已领、东西没发"**。

按优先级：

1. **首选 `u.Mount(&model, ids...)` 把那张表拉进同一个事务**——改动进的是同一个 BulkWrite，
   由框架 Submit 末尾一次提交，真原子，**不需要 Verify**。
   代价是那张表的模型要实现 `updater.MountModel`。细节见 updater 仓库的 `CLAUDE.md`，
   跨项目通用的四条是：

   - 🔴 **挂载走的是与全局句柄相同的 operator 流水线**：`Update`/`Insert`/`Delete` 都只是**入队**，
     verify 才写内存、submit 才进 bulkWrite，请求失败自动回滚。它们返回 `*operator.Operator`
     （nil 表示出错，原因在 `u.Error`），**不是 error**。
   - 🔴 **`Receive` 不是 `Insert`**：前者只把手上已有的记录塞进内存缓存、不产 operator；
     后者产 `TypesNew`，会把**从库里查出来的已有记录当新记录再写一遍**（踩过）。
     条件/批量查询挂载做不到（它只按 `_id` 取数），只能直接查库——但查出来的结果一律
     `Receive` 进去，后续读写才全是缓存命中。
   - **卸载粒度是整张表**：同一玩家可能同时有多条在途记录，摘掉一条会把另一条的缓存一起端掉。
     短命（请求内用完）`defer Unmount`；长命（跨请求，如下单→支付→回来核销）**不卸载**，
     靠玩家下线时 `Destroy` 刷盘，但单条走到终态要 `Remove`，否则长在线玩家会一直堆历史记录。
   - ⚠ **要写的字段必须在模型 schema 里声明**：`dataset.Document.Set` 查不到字段名就**直接丢掉**
     （只打一行 Alert）。直连 DB 写 map 时没人在乎有没有声明，**一改成挂载就会静静少写一个字段**。
     给每张 MountModel 补一个"字段声明齐全"的单测钉住。

2. **退而求其次：直写之前手动 `u.Verify()`**，失败即 `return err`（此时直写还没执行、Updater 也
   随 handler 返回 error 回滚，两边都不落）。⚠ **这只是把窗口缩小、没有关掉**：Verify 过了之后
   落库仍可能失败，而直写那条已经写出去了。只在"那张表实在做不成 MountModel"时用。

3. **不需要管的**：那张表与玩家数据**没有原子性要求**时照旧直写（纯状态位、外部系统状态的本地
   镜像、打点日志）。

⚠ **有一种情况仍然只能直写**：这一趟末尾要 `return error`，但要记的是**已经发生的事实**
（外部平台回了"已取消/核销失败"）。进事务的话它会随 error 一起回滚，那个终态永远记不下来，
每次都要再问一遍外部系统。这类直写之后**内存里那份要跟着同步**（用 ORM 的
"更新后回填整份文档"能力，别照着回包手抄几个字段——以后加字段会漏）。
它**不违反**"取到的指针一律只读"：那条铁律防的是"改了内存却没进库/会被回滚"，
而这里库上一行刚写完、这条改动本来也不该回滚。

## 🔴 iid 推导不出 IType 的集合：模型必须自己覆盖 `IType()`

`Updater.Add/Sub` 是**按 iid 全局路由**到 handle 的。所以一旦某个集合的"iid"其实是**别的业务的
配置表 id**（活动 id、任务 id、商品 id……），路由就不成立，三条同时生效：

1. **模型必须自己实现 `IType(int32) int32`** 返回那个内部号；
2. **不能走 `Updater.Add/Sub`**，要先拿 `u.Collection(IType)` 或对应的业务访问器；
3. 别为了让 iid "推得出来"去污染道具前缀映射表——那张表是给真道具用的。

⚠ 坑在于 `updater.ModelIType` 是**可选**接口，而业务的模型基类通常已经实现了一个
"按 iid 前缀推导"的 `IType()`——**类型断言恒成立、返回值却是 0**。不覆盖就一路静默到运行期才炸：
`Collection` 拿不到 IType，**所有写操作报 `ErrITypeNotExist`**，而同一批集合里早写了覆盖的那些
一切正常，看起来完全像是偶发。

> 真实事故：一张表改名后漏了这个覆盖，整条发货链失败；同批的另外六张表早就都写了。
> **新建这类集合时，把"有没有覆盖 IType"当成 checklist 第一项。**

## 提前拿到"这次发放的最终结果"：优先用 `Player.Verify()`

溢出截断、重复自动分解这类信息在 **Parse 期**才产生，而 Parse 默认发生在 handler 返回
**之后**框架那次 `Submit()` 里，handler 读不到。要在 handler 内读：

- 调 `c.Player.Verify()`（即 `Updater.Verify()`，Player 内嵌提升上来）——跑同一个
  `data→verify` 循环、overflow 照常触发，
  但**不落库、不清 dirty、不发成功事件**，读完让框架照常收尾即可，没有"忘了把推送放回去"
  的风险（库注释明写「Verify 之后再 Submit 是安全的」：status 已被消耗，
  Submit 的收敛循环直接跳过）；
- 只在**接口内还要单独写库、且那张表做不成 `MountModel`** 时才手动 `Submit()`（能挂载就挂载，见上一节）（如领邮件后改邮件状态：先 Verify/Submit
  确认道具发得出去，再写自己那张表，避免"表已改、道具没发"）——此时**务必
  `u.Dirty(ops...)` 把返回值放回去**，见上一节；
- `Verify()` 返回的 error 必须 `return`：Parse 的错误不置 `u.Error`，靠 handler 回非零
  code 才能让框架跳过后续 Submit。吞掉它会落库半成品；
- `Collection.Add(id, value)` 返回 `*operator.Operator`，发放时留住指针，`Verify()` 之后读
  `op.OType` 即可**精确到具体是哪一次发放**（按 iid 事后反查在"同一请求内多次发放同一 iid"
  时分不清是第几次）。

## 🔴 给 `Player` 加字段前，先确认不与 `Updater` 的导出成员重名

`Player` 内嵌 `*updater.Updater`，**同名字段会遮蔽嵌入类型的方法**。历史上踩过两次：

| 旧字段名 | 遮蔽了 | 现名 |
|---|---|---|
| `Verify *verify.Verify` | `Updater.Verify()` | **`Condition`** |
| `Dirty Dirty` | `Updater.Dirty(...)` | **`Pending`** |

当时的症状是 `c.Player.Verify()` 编译失败（`*verify.Verify is not a function`），
所有调用点被迫写 `c.Player.Updater.Verify()` 绕开——注释里到处是「必须写 .Updater.」。
改名后遮蔽消失，`c.Player.Verify()` / `c.Player.Dirty(...)` 直接可用。

好在这类冲突是**编译期**炸，不会静默出错。加字段时对一眼 `Updater` 的导出方法表即可
（`Add/Sub/Get/Val/Set/Select/Submit/Verify/Dirty/Release/Reset/Emit/On/Cache/Error/...`）。

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

> ✅ **updater `76be8e1` 起，走 handle 的那条路已经堵上**：`Document.Field()` 对含 `.` 的
> 子键路径也会校验根字段（此前直接放行），写错字段名现在会置 `Updater.Error`、整个请求失败。
> `dataset.Document.Set` 里的静默返回保留为**直接操作 dataset 时的兜底**，业务经
> `Document.Set` / `doc.Set` 写入不会再遇到。
>
> ⚠ 那次修复只校验、**不改写 key**：`Name()` 返回的是 `JSName()`（PascalCase），拼回子键会把
> `soulrelics.1` 变成 `SoulRelics.1`，而 `update.Transform` 对含 `.` 的 key 原样下发，
> 等于往库里写一个大小写不符的野字段。改这段代码前先看那两条单测。

## 派生结算（"够了就自动升/自动结算"）挂 `EventTypeSubmit`，不要挂 `Listener`

一类很常见的需求：某个量变化后要顺带做点别的——经验够了升级、材料够了自动合成、
按时间补足的资源要结算。**位置选错就必然算错**，两段式是唯一正确的形状：

```text
IType.Listener(u, op)          ← 只做一件事：把"变了的那个 id"记进中间件
    ↓
Middleware.Emit(u, EventTypeSubmit)   ← 在这里读最新值、判定、追加新的 operator
```

- **Listener 里读不到新值**：它跑在 operator **入队那一刻**（`mayChange`），此时还没 Parse，
  读到的是**旧值**——判"够不够"必然算错。而且同一请求里对同一 id 可能 `Add` 多次
  （十连抽、批量发奖），每次都判一遍还会重复触发。
  `EventTypeSubmit` 在 `data→verify` 之后触发，那时 Parse 已把增量应用进内存，读到的才是终值。
- **Listener 里只认"增加"那一类 operator**：派生结算自己也要 `Sub`/`Set`，不过滤就是自我递归。
- **中间件用完即摘**（`Emit` 返回 `false`）：Submit 是**收敛循环**，追加 operator 会让它再跑一轮，
  不摘就空转到 100 轮上限报错。之后再有变化时 Listener 会重新 `LoadOrCreate` 建一个。
- 🔴 **失败必须静默跳过，不能返回 error**：在 Submit 阶段报错会让**整个请求回滚**，
  代价远大于"这次先不结算、下次再说"。所以派生结算内部要**先全量校验再动手**，不留半截操作。
- ⚠ **它只由"变化"驱动**：存量已经够、但本次没有任何变化的玩家不会被触发（改配置门槛后尤其明显）。
  需要兜底就另留一个手动接口，别指望自动那条覆盖全部。

> 用 `u.Middleware.LoadOrCreate(u, name, creator)` 拿中间件实例：同一请求内多次记录复用同一个，
> 天然去重。

---

## 时间与周期：`[Start, Expire)` 左闭右开，判过期一律 `now >= expire`

`times.Expire` / `Cycle.Expire` / `Player.Times.Expire` 返回的**是区间右端点，那一刻本身已不属于
本届**（周期表的口径是"本届结束时刻 == 下届开始时刻"）：

| 判定 | 写法 |
|------|------|
| 仍在有效期 | `now < expire`（DB 侧 `expire > now`） |
| 已过期 | `now >= expire` |

与 JWT `exp`、HTTP `Expires`、Redis `EXPIREAT`、`context.Deadline` 一致。

> **别写成 `expire < now`**：那要到 expire 之后一秒才算过期，整届晚一个单位结束。
> cosgo `d4f8faa` 之前 `Expire` 返回的是"本届最后一纳秒"，秒截断后 1 纳秒被放大成 1 秒，
> 才需要那种写法；现已回归惯例，**旧项目升级 cosgo 后要把所有 `<` 改成 `<=`**。

> **换算成"天"时，减 1 要减在时间轴上、不是减一天**：`end.Add(-1).Sign(0)` —— 让时刻先退回
> "仍属于本届的最后一刻"再取日期签名。expire 不一定落在 0 点，无脑把 end 那天整天排掉，
> 会把当天 0 点到 end 之间的记录全丢掉。

---

## 条件验证（players/condition）

通用条件校验系统，用于任务完成判定、解锁条件检查。配置结构实现 `condition.Target`
（`GetCondition()`/`GetKey()`/`GetGoal()`），默认比较方式 `>=`。

入口是 **`Player.Condition`** 字段（`*condition.Verifier`）；注意别和 `Player.Verify()`
方法混了——后者是 `Updater.Verify()`，跑的是提交前的 data→verify 收敛，两者无关。

> 📌 本包早先叫 `players/verify`、类型叫 `verify.Verify`，常量叫 `verify.ConditionData`——
> 包名与 `Updater.Verify()` 撞词、常量与包名结巴，且游戏侧 `game/verify` 自己也是
> `package verify` 正 import 它。已整体改名，接口方法 `GetCondition()` **保持不变**
> （配置表生成代码在实现它，改了会波及导表链路）。

```go
if cfg.GetCondition() > 0 {
    if err := c.Player.Condition.Verify(cfg); err != nil {
        return err   // 条件不满足
    }
}
c.Player.Condition.Auto(cfg)  // 自动版：内部含预读登记，失败时置 u.Error 让整个请求回退
```

框架内置条件类型（`players/condition/condition.go`）：

| 值 | 常量 | 取值方式 |
|----|------|---------|
| 0 | `condition.TypeNone` | 无条件，直接通过 |
| 1 | `condition.TypeData` | 基础数据 / 日常 / 成就记录 |
| 2 | `condition.TypeEvents` | 即时监听事件（仅任务系统） |
| 9 | `condition.TypeMethod` | 调用注册的 Method 函数 |
| 101 | `condition.TypeWeekly` | 周数据（基于 daily） |
| 102 | `condition.TypeHistory` | 历史数据 |

配置里直接写 `[条件类型, 数据键, 目标值]` 三元组的，用 `condition.Array` 包一层即得 `Target`。

业务可用 `condition.Register(key, handle)` 注册自己的条件类型，**也可以覆盖内置的**
（项目里常把 `TypeData` 重注册成查自己的角色字段）。

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

- **handler 已经持有该玩家的锁**，再对**自己**调一次 `players.Get(c.Uid())` 就是自锁死
  （`sync.Mutex` 不可重入，而且没有超时）。手上现成的 `c.Player` 直接用。
- `Player.Send` 要求调用方持有玩家锁。从 daemon、定时器或**别的**玩家协程里推消息，
  必须套 `players.Get(uid, func(p){ p.Send(...) })`。
- 跨玩家操作（交易、好友）用 `c.Mutex().Lock/Async` 同时锁定多个玩家（框架侧是 `players.Locker`），
  不要嵌套 `players.Get`。

### 🔴 handler 里动别人：唯一入口是 `c.GetPlayer`

handler 要改**别的玩家**（送礼、好友互踢、组队、公会）时只能用 **`c.GetPlayer(uid, handle)`**
（`context/locker.go`）。自己 `players.Get(对方)` 是标准 **ABBA 死锁**：两个玩家互相操作时
双方**永久卡死**——不是超时报错，是这两个玩家此后所有请求都挂住。

`c.GetPlayer` 做的正是避开它：目标是别人时先 `Submit` → operator 转存进 `Player.Pending`
→ `Release` → `Unlock`，**让出自己的锁**再去锁对方，回来重新 `Lock`+`Reset`。

**代价必须知道**：一次 `c.GetPlayer` 会让自己的 Updater 走完整的 Submit → Release → Reset，
**中途真的提交一次数据库**并重新 Emit `EventTypeReset`。它不是"顺便看一眼别人"的轻量操作，
**循环里对一批人挨个调是错的**——那种场合用 `c.Mutex()`（批量锁，一次取齐，不会死锁）。

### `players.Get` / `Load` / `Login` 各自什么时候能用

| | 语义 | 谁能用 |
|---|---|---|
| `players.Get(uid,h)` | 只取在线玩家，不在内存回 `ErrNotOnline` | **只在没有 Context、也不持任何玩家锁的入口第一层**：内网 RPC 接收端、运营接口、daemon |
| `players.Load(uid,init,h)` | 不在线则实时读写库；`init=false` = **只占锁位不加载数据**（回调里 `p.Updater` 是 nil） | 后台直连库改档这类"不读内存数据、但要与该玩家其它操作互斥"的场景 |
| `players.Login(uid,test,meta,h)` | ⚠ **是真正的登录**：`test` 只管写不写库，内部 `init=true` 拉全量 + `Connected` **标在线**（在线数 +1、发 `EventConnect`） | **只有登录接口**。拿它当"补数据"用就是一次**假上线** |

⚠ 第二个参数别看错：`Load` 是 `init`、`Login` 是 `test`，语义完全不同。
"只占锁位"是 `Load(uid,false,h)`，**不是** `Login(uid,false,...)`。

> 真实事故：某项目写过一个 `helper.GetPlayer(c, uid, handle)`，两处都做反了——攥着自己的锁直接
> `players.Get(别人)`（ABBA），离线时用 `players.Login` 把对方**假上线**。它零调用者却比没有更危险：
> 名字一样、签名还更顺手，下一个写社交的人很容易挑中它。**跨玩家这件事由框架统一处理，
> 项目侧不要再造第二个入口。**
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

### handler 返回值语义

| 返回 | 客户端收到 |
|------|-----------|
| `error` | 默认错误码 + 错误信息，**Updater 自动回滚** |
| 框架的 `Error(msg, code...)` | 指定错误码（不指定则默认码），同样回滚 |
| 任意业务数据（`interface{}`） | code=0 + 数据 + 本次数据变更 |
| `[]byte` | 原样透传，跳过 Message 封装 |

推论：**handler 内不需要"预校验余额够不够"再动手**——扣不动时提交期会失败并整体回滚。
把余额校验和实际扣减写成两段，反而多一处会走偏的口径。

## 主动推送：绕开 `Context.Send` 就得自己补两个 metadata

`c.Send`（内部 `Context.NewSender`）会自动补好推送该有的元信息。**凡是自己拼 metadata 往网关投
的推送**（tick 协程、内网 RPC 接收端、daemon）都得自己带，漏一个的症状都极具迷惑性：

| key | 值 | 不带会怎样 |
|---|---|---|
| `_res_flag` | `message.FlagNoreply` | 客户端见到"既不是应答、又没标不必回执"的包会**自动回执**，那条回执进发送队列等一个永远不来的回包 → 若干秒后报"超时未收到响应"。**推几条就多几条超时**，而业务数据确实到了，完全看不出是推送的锅 |
| `_rid` | 触发本次推送的**客户端请求序号** | 客户端靠它把推送归到那一次请求上。丢了不报错，只是客户端分不清"这东西是我刚才那下点击换来的" |

跨服时 `_rid` 要**随 RPC 透传**：触发操作的请求打在 A 服，而真正推送的是 B 服，B 手上没有那个序号。
真正主动的推送（广播、定时投递）本来就没有对应请求，留空即可。

### 🔴 tick / daemon 驱动的推送：路由必须在"拿玩家信息那一刻"一起取走

`c.Send` 在没有 Player 时靠请求 metadata 里的会话标识定位连接——**tick 协程手里根本没有请求**，
这条路从原理上就覆盖不到（日志只有一句"GUID 与 SocketId 均为空"，服务端一切正常、客户端什么都收不到）。

所以：**取玩家信息的那一次（RPC 拉档案 / 持锁那一刻）就把网关地址 + 会话标识一起取回来自己存着**，
之后按它直接投网关。踩过两次，症状都是"服务端日志全对、客户端一片空白"。

推论——**推送本身不属于"玩家数据"**：网关只要「网关地址 + 任一会话标识」就能投到那条连接上，
不需要玩家对象。这让"不碰玩家数据的服"也能自己推消息，不必绕一趟回主服代发。

> ⚠ 会话标识优先级由网关决定（通常"认死一条连接的 id"优先于"账号级 guid"）。
> **认死连接的那个必须取当次请求的活值，不能用早先存下的快照**——顶号/重连之后，
> 上一代的数据会被推给刚上来的另一个人。

### ⚠ 网关的连接池往往是**独立实例**，包级函数一个都不能用

网关一般用 `cosnet.New()` 另起一个实例（而非 `cosnet.Default`），因为它要对所持实例做全局性动作：
关掉默认心跳改由 session 接管、遍历整池计数、注册 Replaced/Disconnect/Authentication 回调。
共用全局池的话，同进程里别人建的连接会被卷进来、被网关的心跳判超时断掉。

由此：`cosnet.Get(id)` / `cosnet.On(...)` 这类**包级函数查的都是 `Default`，在这里恒不生效且不报错**，
必须用网关实例上的同名方法。两处都真踩过：按 id 直投恒查不到（"长连接不在线,消息丢弃"，
而客户端明明连着）；注册在 `Default` 上的错误回调**从来没被触发过**，错误事件静悄悄。

## 多服拆分：同进程能跑、拆进程才炸的一类问题

同一个进程里 `cosgo.Use` 起多个服（主服 + 若干专用服）时，下面几件事**在同进程下全部正常**，
拆进程那天才集体失效，且**没有一个会报编译错或启动错**。写多服的项目把这段当 checklist：

| 症状 | 根因 | 规矩 |
|---|---|---|
| 那个服拿不到玩家、推送哑掉 | 直接 `players.Get(uid)` —— 拆开后那个进程里根本没有这个玩家对象，只会拿到 `ErrNotOnline` | **不碰玩家数据的服不许 import `players`**，一切经 RPC 找主服；连"推一条消息""刷一下心跳"这种不改数据的事也不行 |
| 内网调用报 `can not found any client:<名字>` | 配置的 `[service]` 段没登记本进程能访问哪些服务 | 新加一个服，除了 `cosgo.Use` 还要在配置里登记。**服务端注册好了、代码也编得过，极难往配置上想**（踩过） |
| 回包格式静默退回默认、自定义信封失效 | 序列化/模块的注册靠 `init()`，而那个包只是"恰好被别人 import 进来了" | 每个服在自己的 module 里用**导入锚点**显式钉住（`var _ = xxx.Loading`），不要依赖别人替你把包链接进来 |

### 内网 RPC 的两个坑

**坑 1：内网回包的信封和客户端回包不是一套。**
序列化层要按"这是不是客户端请求"分流：内网调用方拿到的是 RPC 约定的信封，
按客户端那套封装的话，调用方只能读到 code、数据恒空。
踩过 `code=0 data=<nil>`，看着像对方没干活，实际是序列化把数据丢了。
**格式由调用方声明（如 `Accept` 头），服务端不猜**——按返回值类型自动切的话，
哪天有人换了某个接口的返回类型，格式就静默变了，而调用方可能是你改不动的外部系统。

**坑 2：内网 RPC 改了玩家数据，要自己推变更通知。**
客户端请求那条链由序列化层自动把本次 operator 打包成"数据变更"塞进回包，**内网 RPC 没人代劳**。
只推了业务事件、没推变更时的症状是：**东西确实落了库，但客户端本地一件没多、下次登录才冒出来**。

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
| `README.md` | 模块清单、并发模式（每玩家一把互斥锁；actor 为什么被移除） |
| `players/README.md` | 玩家生命周期、7 个状态迁移、事件、内存回收 |
| `options/README.md` | 运行时配置、`Setting` 可插拔函数（GetIType/GetIMax/Renewal） |
| `modules/{rank,graph,chat,locator}/README.md` | 排行榜 / 社交图谱 / 聊天 / 全服定位 |
| `updater` 仓库的 `CLAUDE.md` | **updater 内部实现**：四种数据模型、IType 路由、RAMType、Dirty 追踪、operator 流水线、`Mount` 临时挂载、熔断。改数据层前必读 |

## Language

代码注释、错误信息、文档一律中文，保持此约定。
