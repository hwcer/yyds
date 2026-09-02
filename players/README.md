# Players

玩家管理模块，负责玩家的生命周期管理、并发控制和内存回收。

## 并发模式

**每玩家一把互斥锁**（`Player.Lock/Unlock`），没有第二种模式可选。

| | |
|---|---|
| 同步点 | 每玩家一把 `sync.Mutex`（`Player.Lock/Unlock`） |
| 并发度 | 真并发，吃满多核；业务跑在调用方（rpcx 的请求）协程上 |
| 取得一个玩家 | 拿他那把锁 |
| 取得权限之后能改谁 | **只能改锁住的那几个** |
| 跨玩家操作 | `context.Mutex().Lock/Async` 成批取锁，全服排一条 `await` 队列防 ABBA |

锁就直接内嵌在 `Player` 上（`Player.mutex`），没有中间抽象层 —— 曾经有过一个
`player.Syncer` 接口和一个 `players/locker` 子包用来切换并发实现，随 actor 一起移除了。
要换并发模型的话，入口是 `Player.Lock/Unlock` 加上 `manage.go` / `batch.go` 这两个文件。

## 为什么没有 actor 模式

框架里曾经有过 `AsyncModelActor`，**已整体移除**。加回来之前先读完这一节，
否则会重走一遍已经走过两次的弯路。

### 它返工过两次，两次都是模型错误

1. **每玩家一条 chan + 一个 worker，业务在 worker 里跑**。两个实测可复现的永久性故障：
   `Lock()` 把 `holding` 通道写在共享字段上，后来者覆盖先来者（8 个协程同时"持有"同一玩家，
   随后 `close of closed channel`）；以及「A 的请求取 B」与「B 的请求取 A」各占着自己的
   worker 等对方，双方永久挂起、全程无超时。
2. **收敛成一条全局通道 + 一把全局大锁**。不死锁了，但把全服玩家操作压到**单个协程**——
   连 `Loading` 六张表、`Submit` 的 BulkWrite 都在里面跑，一次慢查询全服停顿；
   而且"排到通道 = 拥有所有玩家的操作权"这套语义与 locker **根本不能互换**；
   还长出独有故障：通道内再进通道（`context.GetPlayer` 就会），自己排在自己身后，
   必然白等一个排队超时然后失败。

### 根因：两个前提，当前装配下都不成立

**① 谁决定业务跑在哪个协程上。** rpcx 对每个请求 `go processOneRequest`
（`rpcx/server/server.go:520`，`cosrpc` 没有注入 `WorkerPool`），执行体已经定了。
actor 再把业务搬进玩家 worker，等于一个在途请求占**两个协程 + 两次调度**，是净亏。
要翻盘就得拿到 dispatch 权——而 `WorkerPool.Submit(task func())` 只给一个不透明闭包，
看不到 `req`，也就不知道该投给哪个玩家；要按 uid 路由只能在 auth 插件里偷存，
强耦合 rpcx 的内部调用顺序。

**② 框架 API 得允许异步。** `players.Get(uid, handle) error` 是同步的（原地回调、
阻塞调用方、要拿返回值）。而 actor 的排他性绑在**执行体**上：业务正跑在这个玩家的 worker 里，
"让出"只能靠从消息里 return，那等于丢掉调用栈。于是"取得别人 → 拿到结果 → 接着往下写"
这类**同步跨玩家**写法在 actor 下无解——让出锁也没用，锁本来就不是那道门。

> locker 的排他性绑在**数据**上：持有者是调用方协程，可以中途 `Unlock` 让出，调用栈不动。
> 这正是 `context.Mutex().Lock` 与 `GetPlayer` 能成立的全部原因。

### 那它还剩什么

只剩 FIFO 公平（等待者按先来后到唤醒，`sync.Mutex` 是 barging）。而这一条用
**通道令牌**就能拿到——`Lock()` 从容量 1 的通道取走令牌、`Unlock()` 放回，
Go 在 send 时把值直接交给队首等待者，后到的抢不走；零额外协程、业务栈还在请求协程里。
也就是说，per-player **worker 协程**买到的每一样东西，一把锁或一个令牌通道都给得起。

### 什么时候值得重做

三条同时满足：拿到了 dispatch 权（换传输层，或 rpcx 按 uid 路由）、框架跨玩家 API 改成异步、
并且确实需要玩家自己的事件循环（战斗循环、玩家内定时器）。缺一条就会退化成上面两次的样子。

### 批量取得多个玩家

`context.Mutex().Lock/Async` 只保证「取得操作权限」，**不保证玩家在线、也不保证有数据**：

- `p.Updater != nil` — 玩家在内存（在线，或离线未回收），已 Reset，可正常读写；
- `p.Updater == nil` — 玩家不在内存（必然离线），这是**空壳**，直接读写就是当场 nil panic。

空壳是设计不是缺陷：批量锁的典型用途是"改对方一两个字段、发条消息"，而 `Loading` 会把该玩家
全部常驻模型整表拉一遍并长期留在内存。需要数据时按需加载（`p.Initialize()`），
只改一两个字段就直连 DB —— 完整契约见 `player.Locker` 接口上的注释。

## 玩家状态

```
StatusNone(0) → StatusConnected(2) → StatusDisconnect(3) → StatusOffline(4) → StatusReleased(5)
```

| 状态 | 值 | 说明 |
|------|---|------|
| StatusNone | 0 | 初始状态，仅被加载到内存（启动预加载或异步读取），从未上线 |
| StatusLocked | 1 | 临时锁定，Loading 期间的中间状态 |
| StatusConnected | 2 | 在线 |
| StatusDisconnect | 3 | 连接断开，等待重连 |
| StatusOffline | 4 | 掉线，进入回收队列，此时上线还能抢救 |
| StatusReleased | 5 | 正在释放资源，无法进行任何操作 |
| StatusTerminated | 6 | 被强制下线（`Terminate`），拒绝一切请求，等 daemon 释放 |

## 生命周期事件

| 事件 | 触发时机 |
|------|---------|
| EventConnect | 玩家首次上线 |
| EventReconnect | 同网关断线重连 |
| EventReplace | 不同网关顶号 |
| EventDisconnect | 心跳超时，连接断开 |
| EventOffline | 断开连接超时，业务层面掉线 |

## 回收机制

### 守护协程

`daemon` 协程随 `Start()` 启动，每 `Heartbeat`（默认 5s）执行一次 `worker` 扫描，负责检测玩家状态变化和内存回收。定时器会扣除 `worker` 自身耗时，使扫描周期稳定在 `Heartbeat` 而非 `Heartbeat + worker 耗时`。服务关闭时自动执行 `shutdown` 保存所有玩家数据。

`worker` 分两段：**先快照、后投递**。`ps.Range` 内只做状态判定并把待迁移玩家收进切片；Range 返回、`Manage` 读锁释放之后，再把 `disconnect` / `offline` / `released` 投给状态迁移执行池（见下方「状态迁移执行池」），daemon 自己只留一个不抢锁的 `recycling`。

分两段的原因：这些迁移要抢玩家锁并触发业务事件（可能再抢其他全局锁），在持有 `Manage` 读锁时等细粒度锁，会把需要 `Manage` 写锁的登录路径整体挡住，批量掉线时形成全局停顿。快照与投递之间状态变了不要紧——这几个函数内部都用 CAS 二次校验，会安全跳过。

### 状态流转

在线玩家需要经过完整的状态流转，每一步都有独立的超时计时：

```
Connected ──(ConnectedTime 120s 无心跳)──→ Disconnect    触发 EventDisconnect
                                              │
                                     (DisconnectTime 120s)
                                              │
                                              ↓
                                          Offline         触发 EventOffline
                                              │
                                       (OfflineTime 60s)
                                              │
                                              ↓
                                        Recycling Map     等待内存回收
                                              │
                                         (内存压力触发)
                                              │
                                              ↓
                                     Released → Destroy    释放资源，从内存移除
```

每次状态转换都会重置心跳时间（`KeepAlive`），下一阶段的计时从零开始。从最后一次心跳到进入回收站，最少需要 **300 秒（5 分钟）**。

在此期间玩家随时可以重新连接，状态会跳回 `Connected`。

### 强制下线（Terminate）

`players.Terminate(p)` 把玩家迁入 `StatusTerminated`，这是**拒绝态**——`player.Denied` 会把它和 `StatusLocked` / `StatusReleased` 一起判为拒绝访问，接入层据此拒绝该玩家的一切请求，`Connected()` 也不再让他上线。欠着的 `EventDisconnect` / `EventOffline` 当场按原状态补发（`Connected` 欠两个、`Disconnect` 只欠 `Offline`、`None` / `Offline` 不欠），`worker` 下一个 tick 直接 `released`，**不走 `Disconnect` / `Offline` 的宽限期**。

```text
任意可上线状态(None/Connected/Disconnect/Offline) ──Terminate──→ Terminated ──(下一个 tick)──→ Released
```

三个要点：

- **必须用状态，不能只改心跳**。接入层在任何判断之前就会 `KeepAlive` 刷新心跳，只改心跳的话玩家发一个包就把踢人无声撤销了。
- **必须覆盖所有可上线状态**，不能只踢 `Connected`。`Connected()` 允许从 `None` / `Disconnect` / `Offline` 复活，掉线但仍在内存里的玩家会顶回来。
- **调用方必须持有玩家锁**，与 `player.Send` 同一契约——补发事件要碰 `Updater`。业务层操作 `c.Player` 时天然满足；踢别人套一层 `Get(uid, func(p){ Terminate(p); return nil })`。不要改成调 `disconnect()`，它内部还会 `p.Lock()` 一次，mutex 不可重入。

`Terminate` 只终结当前会话，释放完成后重新登录是允许的。永久封禁要靠业务侧的落库标记（如 `role.ban`）在登录路径上拦。

### StatusNone 特殊处理

`StatusNone` 是从未上线的玩家（启动预加载或异步读取到内存），不走 `disconnect → offline` 流程，不触发任何事件。心跳超时后由 `recycling()` 直接将状态设为 `StatusOffline` 并加入回收站。

### 内存回收策略

回收站中的玩家不会立即被销毁，而是根据内存压力按需释放：

```
触发条件: 缓存总数 >= MemoryPlayer(2000) + MemoryRelease(100)
释放顺序: 按心跳时间升序，优先释放最久未活跃的玩家
释放目标: 将缓存总数降至 MemoryPlayer 以下
分批释放: 每个 tick 释放一批 ReleaseBatch(100) 个，没轮到的顺延到下个 tick
```

两个旋钮各管一件事：`MemoryRelease` 是**触发余量**（超出 100 才开始清理，留这段是为了避免在阈值上反复横跳），`ReleaseBatch` 是**一批的大小**（每个 tick 清 100 个）。

分批是因为每个脏玩家一次 BulkWrite——一次退潮把上千个的落库同时压给数据库不是好主意，状态迁移执行池的队列也吃不下。⚠ `ReleaseBatch` 的零值是致命的（一个都放不掉，内存只涨不跌），`Start` 里有兜底。一批放满了仍有积压时会打一条 `logger.Trace`，避免从监控指标上看像是「清不动」。

释放过程：CAS 置 `StatusReleased`（接受 `StatusOffline` / `StatusTerminated`）→ 取玩家锁（等待在途业务调用结束）→ `Reset`（重置数据）→ `Destroy`（销毁 Updater、清理内存）→ 解锁 → 从管理器中删除。如果 `Destroy` 失败，状态还原成进来时那个，下次重试。

顺序上有两个约束不能调换：

- **必须持锁才能 `Destroy`**：`Destroy` 会把 `Updater` 置空，不加锁就可能在业务协程执行 `handle` 期间抽走它，对端 `defer Release()` 直接 nil panic。相应地 `Get` 也必须**先判 `Denied` 再 `Reset`**。
- **CAS 放在取锁之前**：状态提前翻成 `StatusReleased`，后来的 `Get` 拿到锁就能立刻按拒绝态返回，不必排队等一把注定用不上的锁；而 `Destroy` 仍在锁内，不会和正在跑的业务协程撞上。
- **失败时不能硬编码还原成 `StatusOffline`**：`StatusTerminated` 退化成 `Offline` 会被 `Connected()` 复活，踢/封标记就丢了。
- **`ps.Delete` 必须在解锁之后**：`Delete` 要取管理器写锁，持着玩家锁去抢它，会和"持管理器读锁等玩家锁"的一方成环（`shutdown` 的 `Range` 就是那一方）。这也是这段不能用 `defer` 解锁的原因。

### 状态迁移执行池

`daemon` 是**单协程**，而三种迁移都要抢玩家锁、都可能很慢（`disconnect`/`offline` 要触发业务
的下线事件，`released` 还要在锁内做一次 BulkWrite 落库）。串行跑的话，一次退潮 N 个玩家就是
N 次「等锁 + DB 往返」首尾相接，**扫描周期直接被最慢的那个玩家决定**。

所以 daemon 只负责判断"谁该迁移到哪个状态"，迁移动作本身投给执行池（`migrate.go`）。
三条性质让这件事安全，缺一条都不能这么改：

| | |
|---|---|
| CAS 守卫 | 三个迁移函数第一件事都是 CAS 翻状态，重复投递是空操作 |
| 互不相干 | 玩家之间独立，谁先谁后不影响结果 |
| 可重试 | 没投出去、或执行失败的，状态不变，下一个 tick 会重新收集到 |

**投递非阻塞**（`select + default`）——阻塞就违背了整件事的目的。队列满时放弃本轮投递，
并打一条 `Trace`；持续出现说明数据库慢或 `MigrateWorker` 该调大了。

`recycling` 是唯一留在 daemon 协程里的迁移：它只往回收站那张 daemon 私有的表里记一笔，
不抢任何玩家锁、也不碰 `Updater`，投出去反而要给那张表加锁。

### 优雅关闭

收到退出信号时 `shutdown` 会：

1. 翻转启动状态，此后所有请求返回 `ErrServerClosed`（兼作幂等守卫）
2. `migrateWait()` 等执行池排空并退出 —— 那批在途任务正持着玩家锁在补下线事件、在落库
3. `migrateDrain()` 兜底：ctx 触发时 daemon 可能正好在投递，而池已经退出，那批任务谁都不会碰
4. `Range` **只做快照**，一个玩家锁都不碰
5. 循环外补下线事件：`Connected → disconnect + offline`、`Disconnect → offline`、`None → CAS(Offline)`
6. `releaseAll` 并行释放并等齐，返回未成功数并写进日志

第 4 步是硬约束：`Range` 持有管理器读锁，而 `released` 要回头 `ps.Delete`（写锁）——
在 `Range` 里抢玩家锁就会成环（这里持读锁等玩家锁，那把锁的持有者在等写锁）。
"服务器都在关了哪还有锁"不成立：`scc` 的 ctx Done 不会让在途请求立刻结束。

第 5 步用 CAS 而不是无条件 `Store(Offline)`：后者会把 `Released`（刚被池释放完）和
`Terminated`（被踢，`released` 本就接受）一并抹掉。

## 配置参数

```go
players.Options.Heartbeat      = 5    // 守护协程扫描间隔（秒）
players.Options.ConnectedTime  = 120  // Connected 状态无心跳超时（秒）
players.Options.DisconnectTime = 120  // Disconnect 状态超时（秒）
players.Options.OfflineTime    = 60   // Offline 状态进入回收站超时（秒）
players.Options.MemoryPlayer   = 2000 // 常驻内存玩家数量
players.Options.MemoryRelease  = 100  // 回收触发余量，缓存 >= MemoryPlayer + MemoryRelease 时开始清理
players.Options.ReleaseBatch   = 100  // 每个 tick 释放一批的大小
players.Options.MigrateWorker  = 4    // 状态迁移执行池并发度
```

批量锁那条队列的深度与排队超时**不是配置项**（`manage.go` 里的 `batchCap` / `batchTimeout`）——
它们不是需要按项目调的旋钮：队列深度纯粹是突发吸收能力，调大无风险也无收益；排队超时的语义是
「愿意在队列里排多久」而不是「任务最多跑多久」，调大只是把「快速失败」换成「客户端干等」。
而真正想提吞吐的人会盯上的那个旋钮——worker 数——恰恰不是选项，单 worker 就是防 ABBA 机制本身。

`MigrateWorker` 的瓶颈在**数据库**而不是 CPU（`released` 每个玩家一次 BulkWrite），
别按核数设，按库扛得住多少并发写来设；停服时的全量落库用的也是这个值。

## 代码结构

```
players/
├── default.go     // 入口，Start/Get/Load/Login/Locker 等公开 API
├── manage.go      // 玩家管理器 + 单玩家取用（get / load）
├── batch.go       // 批量锁：同时取得多个玩家（context.Mutex 的底座）
├── service.go     // 可服务性判定（启动状态 + 维护开关）
├── daemon.go      // 守护协程：扫描收集状态迁移、优雅关闭
├── migrate.go     // 状态迁移执行池（daemon 只收集，迁移在这里跑）
├── preload.go     // 启动时预加载活跃玩家
├── options.go     // 配置参数
├── emitter.go     // 生命周期事件定义
└── player/        // Player 结构体、状态常量、玩家锁、核心方法
```
