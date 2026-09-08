package yyds

import "github.com/hwcer/yyds/config"

// 配置门面：转发到 config 包的快照访问函数。
// 旧版是 `var Config = config.Config` 的实例门面(热更时整体换字段，无同步)，
// config 包改为原子快照发布后，这里只保留函数转发。

type Snapshot = config.Snapshot
type CS = config.Snapshot //旧名兼容，同 Snapshot
type Handle = config.Handle

func Load() *config.Snapshot { return config.Load() }

func Is(iid int32, it ...int32) bool { return config.Is(iid, it...) }

func Has(k int32) bool { return config.Has(k) }

func GetIMax(iid int32) int64 { return config.GetIMax(iid) }

func GetIType(iid int32) int32 { return config.GetIType(iid) }

func GetName(iid int32) string { return config.GetName(iid) }

func Reload(data any, path string) error { return config.Reload(data, path) }

func Register(i ...Handle) { config.Register(i...) }
