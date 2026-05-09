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

`daemon` 协程随 `Start()` 启动，每 `Heartbeat`（默认 5s）执行一次 `worker` 扫描，负责检测玩家状态变化和内存回收。服务关闭时自动执行 `shutdown` 保存所有玩家数据。

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

### StatusNone 特殊处理

`StatusNone` 是从未上线的玩家（启动预加载或异步读取到内存），不走 `disconnect → offline` 流程，不触发任何事件。心跳超时后由 `recycling()` 直接将状态设为 `StatusOffline` 并加入回收站。

### 内存回收策略

回收站中的玩家不会立即被销毁，而是根据内存压力按需释放：

```
触发条件: 缓存总数 >= MemoryPlayer(2000) + MemoryRelease(100)
释放顺序: 按心跳时间升序，优先释放最久未活跃的玩家
释放目标: 将缓存总数降至 MemoryPlayer 以下
```

释放过程：`Reset`（重置数据） → `Destroy`（销毁 Updater、关闭 Syncer、清理内存） → 从管理器中删除。如果 `Destroy` 失败，状态回退到 `StatusOffline`，下次重试。

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
