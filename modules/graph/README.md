# 社交图谱 (Graph) 模块

## 项目简介

社交图谱模块是一个用 Go 语言实现的高效社交关系管理系统，主要用于管理用户之间的社交关系，包括关注、好友、粉丝和黑名单等功能。

该模块设计简洁、性能优良，支持高并发操作，适用于各种需要管理用户关系的应用场景。

## 功能特点

- ✅ **完整的关系管理**：支持关注、好友、粉丝、黑名单等多种关系类型
- ✅ **并发安全**：使用读写锁保证高并发场景下的数据一致性
- ✅ **批量操作**：支持批量关注、批量接受好友请求等操作
- ✅ **消息广播**：支持向好友群体广播消息
- ✅ **灵活扩展**：通过 Factory 接口支持自定义业务逻辑
- ✅ **内存高效**：针对大规模用户场景进行了内存优化
- ✅ **性能优良**：采用高效的数据结构和算法

## 安装

```bash
go get -u github.com/yourusername/yyds/modules/graph
```

## 快速开始

### 1. 实现 Factory 接口

```go
import (
    "github.com/hwcer/cosgo/values"
)

type MyFactory struct{}

func (f *MyFactory) Create(uid string) (values.Values, error) {
    // 创建用户并返回业务数据
    return values.Values{"name": "User" + uid}, nil
}

func (f *MyFactory) SendMessage(uid string, path string, data any) {
    // 发送消息实现
    fmt.Printf("Send message to %s: %s\n", uid, data)
}

func (f *MyFactory) Limit(uid string) int32 {
    // 返回好友上限
    return 500
}

func (f *MyFactory) SetPlayer(uid string, key string, value any) {
    // 持久化玩家信息
}
```

### 2. 创建社交图谱

```go
import "github.com/yourusername/yyds/modules/graph"

factory := &MyFactory{}
g, _ := graph.New(factory)
```

### 3. 基本操作示例

```go
// 添加用户
g.Add("user1")
g.Add("user2")

// 关注用户
g.Follow("user1", []string{"user2"})

// 成为好友（互相关注）
g.Follow("user2", []string{"user1"}) // 自动成为好友

// 检查好友关系
isFriend := g.Has("user1", "user2")
fmt.Println("Are friends:", isFriend) // 输出: true

// 遍历好友
g.Range("user1", graph.RelationFriend, func(getter graph.Getter) bool {
    fmt.Println("Friend:", getter.Fid())
    return true
})

// 广播消息
g.Broadcast("user1", "notification", "Hello friends!")
```

## 核心 API

### 初始化与管理

- **New(factory Factory) (*Graph, Install)**：创建新的社交图谱
- **Add(uid string) error**：添加新用户

### 关系管理

- **Follow(uid string, fid []string) *Result**：关注用户
- **Accept(uid string, tar []string) *Result**：接受好友请求
- **Remove(uid string, fid []string)**：移除好友关系
- **Unfriend(uid string, fid string)**：拉黑用户

### 关系查询

- **Has(uid string, fid string) bool**：检查是否是好友
- **Relation(uid string, fid string) Relation**：查询关系类型
- **Count(uid []string, t Relation) map[string]int32**：统计关系数量

### 遍历与操作

- **Range(uid string, relation Relation, handle func(Getter) bool)**：遍历好友
- **Broadcast(uid string, name string, data any)**：广播消息

## 关系类型

| 关系类型 | 值 | 描述 |
|---------|-----|------|
| RelationNone | 0 | 无关系 |
| RelationFans | 1 | 我的粉丝 |
| RelationFollow | 2 | 我的关注 |
| RelationFriend | 4 | 我的好友 |
| RelationUnfriend | 128 | 黑名单 |

## 性能说明

### 内存占用
- **2万用户，每个用户100个好友**：约占用135MB内存
- **主要内存消耗**：Friend对象、friends map、字符串内存

### 并发性能
- **读操作**：使用读锁，支持高并发
- **写操作**：使用写锁，保证数据一致性
- **锁持有时间**：优化了锁的使用，减少锁竞争

### 时间复杂度
- **添加用户**：O(1)
- **关注用户**：O(n)，n为目标用户数量
- **检查关系**：O(1)
- **遍历好友**：O(n)，n为好友数量

## 高级特性

### 自定义 Factory

通过实现 Factory 接口，可以自定义：
- 用户创建逻辑
- 消息发送方式
- 好友数量上限
- 玩家信息持久化

### 扩展关系类型

可以通过扩展 Relation 类型添加自定义关系：

```go
const (
    RelationFamily Relation = 1 << 3     // 家人
    RelationColleague Relation = 1 << 4  // 同事
)
```

### 性能优化

对于大规模应用，可以考虑：
- 实现用户对象的惰性加载和过期清理
- 使用分片存储减少单个 map 的大小
- 实现批量操作的并行处理
- 添加缓存层减少数据库访问

## 错误处理

常见错误类型：

| 错误 | 描述 |
|------|------|
| ErrorAddYourself | 尝试添加自己为好友 |
| ErrorYourUnfriend | 被对方拉黑 |
| ErrorAlreadyFriend | 已经是好友 |
| ErrorAlreadyFollow | 已经申请过 |
| ErrorTargetUnfriend | 对方拉黑了你 |
| ErrorYourFriendMax | 好友数量达到上限 |
| ErrorTargetFriendMax | 对方好友数量达到上限 |

## 示例应用

### 社交网络

管理用户之间的关注、好友关系，支持消息通知和社交互动。

### 游戏社交系统

处理游戏内的好友关系、黑名单，支持组队邀请和游戏内消息广播。

### 社区平台

管理用户之间的互动关系，支持关注作者、私信通知等功能。

### 企业协作工具

处理企业内部员工之间的组织关系，支持部门消息广播和团队协作。

## 贡献指南

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 打开 Pull Request

## 许可证

本项目采用 MIT 许可证，详情请参阅 [LICENSE](LICENSE) 文件。

## 联系方式

- 项目地址：[https://github.com/yourusername/yyds/modules/graph](https://github.com/yourusername/yyds/modules/graph)
- 问题反馈：[https://github.com/yourusername/yyds/issues](https://github.com/yourusername/yyds/issues)

---

**感谢使用社交图谱模块！** 🎉