# options

## 文件说明

| 文件 | 说明 |
|------|------|
| `options.go` | 全局运行时配置（Options），服务类型常量，Redis 服务发现初始化 |
| `game.go` | 游戏服配置（区服 ID、开服时间、Redis/MongoDB 等） |
| `master.go` | 中控 Master HTTP 客户端，支持 OAuth 签名 |
| `setting.go` | 可插拔的配置函数（GetIMax、GetIType、Renewal），通过函数指针允许外部覆盖 |

## Setting 可插拔配置

`options.Setting` 提供可被外部覆盖的默认实现：

```go
var Setting = struct {
    Renewal  string                              // 跨天路由路径
    GetIMax  func(iid int32) (r int64)           // 道具最大堆叠数
    GetIType func(iid int32) (r int32)           // 道具类型
}{...}
```

在 `init.go` 中注册到 updater：

```go
updater.Config.IMax = options.Setting.GetIMax
updater.Config.IType = options.Setting.GetIType
```

游戏子项目可在模块中覆盖 `options.Setting.GetIMax` 和 `options.Setting.GetIType` 实现自定义逻辑。

