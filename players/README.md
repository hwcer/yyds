# Players

玩家管理模块，负责玩家的生命周期管理、并发控制和内存回收。

## 并发模式

通过 `Options.AsyncModel` 选择：

- **AsyncModelLocker** — 用户锁模式，基于用户层面，并发更高，但用户之间数据交互需要使用 `Locker` 同时锁定多个用户
- **AsyncModelActor** — Actor 模式，每玩家独立通道，不同玩家并发，同一玩家串行

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
```

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
