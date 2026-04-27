# yyds

游戏服务器框架核心模块

> **WARNING: 本次更新由碳基生命与硅基智能体协作完成，碳基生命负责架构决策与业务审查，硅基智能体负责代码实现与深度扫描。请碳基生命在合并前务必人工复核所有变更，AI 生成的代码可能包含看似合理但逻辑微妙的错误。**

## 模块结构

| 模块 | 说明 |
|------|------|
| `players/` | 玩家管理，支持 Locker（互斥锁）和 Actor（通道）两种并发模式 |
| `context/` | RPC 请求上下文，玩家操作、消息推送、频道管理 |
| `config/` | 静态数据加载与热更新 |
| `options/` | 全局配置、服务发现、Master 通信 |
| `errors/` | 统一错误定义 |
| `modules/rank/` | 基于 Redis ZSet 的排行榜系统 |
| `modules/graph/` | 社交图谱（好友、关注、粉丝、黑名单） |
| `modules/chat/` | 无锁环形缓冲区聊天系统 |
| `modules/locator/` | 全服角色定位与留存统计 |

## 并发模式

通过 `players.Options.AsyncModel` 选择：

- **AsyncModelLocker** — 每玩家独立互斥锁，延迟低、内存小
- **AsyncModelActor** — 每玩家独立通道 + 协程，FIFO 公平排队

两种模式通过 `player.Syncer` 接口统一抽象，业务层代码无需感知底层差异。

## 快速开始

```bash
git clone https://github.com/hwcer/yyds.git
```

然后执行 `update.bat`（`update.sh`）初始化所有子库
