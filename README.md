# yyds

游戏服务器框架核心模块

> **WARNING: 本次更新由碳基生命与硅基智能体协作完成，碳基生命负责架构决策与业务审查，硅基智能体负责代码实现与深度扫描。请碳基生命在合并前务必人工复核所有变更，AI 生成的代码可能包含看似合理但逻辑微妙的错误。**

## 模块结构

| 模块 | 说明 |
|------|------|
| `players/` | 玩家管理：生命周期、并发控制（每玩家一把互斥锁）、内存回收 |
| `context/` | RPC 请求上下文，玩家操作、消息推送、频道管理 |
| `config/` | 静态数据加载与热更新，IType/IMax 委托 options.Setting |
| `options/` | 运行时配置（Game/Master/Setting）、Redis 服务发现、服务类型定义 |
| `errors/` | 统一错误定义 |
| `modules/rank/` | 基于 Redis ZSet 的排行榜系统 |
| `modules/graph/` | 社交图谱（好友、关注、粉丝、黑名单） |
| `modules/chat/` | 无锁环形缓冲区聊天系统 |
| `modules/locator/` | 全服角色定位与留存统计 |

## 并发模式

**每玩家一把互斥锁**（`Player.Lock/Unlock`），业务跑在 rpcx 的请求协程上，同一玩家串行、
不同玩家真并行。跨玩家用 `c.Mutex().Lock/Async` 成批取锁，全服排一条队列防 ABBA。

框架里曾经有过 `AsyncModelActor`，已整体移除 —— 在当前装配下它是净亏（rpcx 已经每请求
一个协程，再搬进玩家 worker 等于多一跳），理由与"什么时候值得重做"见
[players/README.md](players/README.md#为什么没有-actor-模式)。

锁直接内嵌在 `Player` 上，没有中间抽象层；容器与批量锁是 `players/manage.go`、`players/batch.go`。

## 快速开始

```bash
git clone https://github.com/hwcer/yyds.git
```

然后执行 `update.bat`（`update.sh`）初始化所有子库
