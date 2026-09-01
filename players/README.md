# Players

玩家管理模块，负责玩家的生命周期管理、并发控制和内存回收。

## 并发模式

通过 `Options.AsyncModel` **二选一**，同一进程只有一个生效。两者的区别不是性能调优，
而是**跨玩家操作要不要自己管同步**：

| | AsyncModelLocker（默认） | AsyncModelActor |
|---|---|---|
| 同步点 | 每玩家一把互斥锁 | **一条全局玩家通道**（`actor` 包的 `w`） |
| 并发度 | 真并发，吃满多核 | **并发绑死在一个协程内**，全服玩家操作串行 |
| 取得一个玩家 | 拿他那把锁 | 排队进入全局通道 |
| 取得权限之后能改谁 | **只能改锁住的那几个** | **任意玩家，随便改** |
| 跨玩家操作 | 要用 `context.Mutex()` 成批取锁，取锁顺序 / ABBA / 空壳都要自己盯 | 直接写，不可能死锁、不可能漏锁 |

### Actor 模式：一个传统编程思想 + Go 特性的模型

**唯一的好处**：一旦取得数据操作权限，就可以修改**任意**玩家的数据 —— 跨角色操作极其方便，
像写单线程程序一样。
**最大的特点**：把并发**绑死在一个协程内**，吞吐就是这一个协程的吞吐。

它的全部同步语义只有一条：

> **排到全局通道 = 取得了所有玩家的操作权限**

由此推论（也是最容易被误解的三点）：

1. **没有"每个玩家一条通道"这回事。** 曾经有过，是错的 —— 见下方"历史教训"。
2. **通道内不需要、也不应该再对目标逐个上锁**（`actor.Locker.loading` 一把锁都不取）。
3. **通道内不能再进一次通道**：只有一个 worker，自己排在自己身后永远轮不到，
   只会白等一个 `LockerTimeout`。`context.GetPlayer` 这类"在业务里再调 `players.Get`"
   的写法在 actor 下就靠这个超时兜底，不会永久挂死，但会白付 5 秒。

`Player.Lock/Unlock` 在 actor 下落到一把全局 `gate` 上 —— 它只用来跟 **不走通道的协程**
（daemon 回收玩家、`Terminate`）互斥，不承担业务并发控制。

#### 历史教训（2026-09-01 修）

早先 `actor.Syncer` 被实现成"每玩家一条 chan + 一个 worker 协程"，与上面的模型南辕北辙，
带来两个**实测可复现、且都是永久性故障**的 bug（回归用例见 `actor/actor_test.go`）：

1. `Lock()` 把 `holding` 通道写在**共享字段**上，两个协程同时取得同一玩家时后者覆盖前者
   —— 实测 8 个协程同时"持有"同一个玩家，随后 `close of closed channel` panic；
2. 业务 handler 跑在**玩家自己的 worker** 上，于是「A 的请求取 B」与「B 的请求取 A」
   各占着自己的 worker 等对方 —— 双方永久挂起，全程无超时。

两条的根是同一个：**把同步做成了 per-player**。收敛回全局通道之后都不复存在。

### 批量取得多个玩家（两种模式共同的契约）

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

`worker` 分两段：**先快照、后迁移**。`ps.Range` 内只做状态判定并把待迁移玩家收进切片，Range 返回、`Manage` 读锁释放之后才逐个执行 `disconnect` / `offline` / `recycling` / `released`。原因是后两者要抢玩家锁并触发业务事件（可能再抢其他全局锁），在持有 `Manage` 读锁时等细粒度锁，会把需要 `Manage` 写锁的登录路径整体挡住，批量掉线时形成全局停顿。快照与迁移之间状态变了不要紧——这几个函数内部都用 CAS 二次校验，会安全跳过。

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
单次上限: 每个 tick 最多释放 MemoryRelease(100) 个，超出部分顺延到下个 tick
```

`MemoryRelease` 同时充当触发余量和单次释放上限——「超出 100 才清理，每次最多清 100」。加上限是因为释放要抢玩家锁，脏玩家还会产生一次 BulkWrite 往返；不限量的话大批量退潮会把 daemon 协程阻塞到秒级，连带拖慢状态机的判定精度。命中上限且仍有积压时会打一条 `logger.Trace`，避免从监控指标上看像是「清不动」。

释放过程：CAS 置 `StatusReleased`（接受 `StatusOffline` / `StatusTerminated`）→ 取玩家锁（等待在途业务调用结束）→ `Reset`（重置数据）→ `Destroy`（销毁 Updater、清理内存）→ 解锁 → 从管理器中删除 → `Close`（关闭 Syncer）。如果 `Destroy` 失败，状态还原成进来时那个，下次重试。

顺序上有两个约束不能调换：

- **必须持锁才能 `Destroy`**：`Destroy` 会把 `Updater` 置空，不加锁就可能在业务协程执行 `handle` 期间抽走它，对端 `defer Release()` 直接 nil panic。相应地 `Get` 也必须**先判 `Denied` 再 `Reset`**。
- **CAS 放在取锁之前**：状态提前翻成 `StatusReleased`，后来的 `Get` 拿到锁就能立刻按拒绝态返回，不必排队等一把注定用不上的锁；而 `Destroy` 仍在锁内，不会和正在跑的业务协程撞上。
- **失败时不能硬编码还原成 `StatusOffline`**：`StatusTerminated` 退化成 `Offline` 会被 `Connected()` 复活，踢/封标记就丢了。
- **`Close` 必须在解锁之后**：actor 模式下 `Syncer.Close` 关的是玩家通道，持锁（通道里挂着栅栏函数）时关闭会让通道 worker 永久阻塞。

### 优雅关闭

收到退出信号时，`shutdown` 会：

1. 将 `playersStarted` 设为 0，拒绝所有新请求（返回 `ErrServerClosed`）
2. 在线玩家走完 `disconnect → offline` 流程，触发对应事件
3. 其余状态的玩家强制设为 `StatusOffline`
4. 遍历所有玩家执行 `released` 释放资源

## 配置参数

```go
players.Options.Heartbeat      = 5    // 守护协程扫描间隔（秒）
players.Options.ConnectedTime  = 120  // Connected 状态无心跳超时（秒）
players.Options.DisconnectTime = 120  // Disconnect 状态超时（秒）
players.Options.OfflineTime    = 60   // Offline 状态进入回收站超时（秒）
players.Options.MemoryPlayer   = 2000 // 常驻内存玩家数量
players.Options.MemoryRelease  = 100  // 回收站阈值，缓存 >= MemoryPlayer + MemoryRelease 时开始清理
players.Options.LockerCap      = 128  // 批量锁 / 全局通道的排队深度
players.Options.LockerTimeout  = 5*time.Second // 【排队】超时，不是「任务最多跑多久」
```

`LockerTimeout` 的语义是**「愿意在队列里排多久」**：`await.Message.Wait` 在 handler 已经开跑时
会重置计时器继续等，跑起来的任务不会被打断。所以超时 ⇔ 任务**一次都没执行**。
调大的代价在 `Mutex().Lock` 那条同步路径：把「快速失败」换成「客户端干等」。

## 代码结构

```
players/
├── default.go     // 入口，Start/Get/Load/Login 等公开 API
├── daemon.go      // 守护协程，状态流转、回收、关闭
├── options.go     // 配置参数、并发模式定义
├── emitter.go     // 生命周期事件定义
├── player/        // Player 结构体、状态常量、核心方法
├── locker/        // Locker 并发模式实现
└── actor/         // Actor 并发模式实现
```
