package players

import (
	"github.com/hwcer/yyds/players/emitter"
)

const (
	EventConnect int32 = -iota - 1
	EventReplace
	EventReconnect
	EventDisconnect
	EventOffline
)

// 全局事件

func On(t int32, handle emitter.EventsFunc) {
	emitter.Events.Listen(t, handle)
}
func Listen(t int32, handle emitter.EventsFunc) {
	emitter.Events.Listen(t, handle)
}

// SetFilter 全局任务条件判断方式
func SetFilter(t int32, f emitter.FilterFunc) {
	emitter.Filters.Register(t, f)
}

// SetMonitor 注册事件监控，触发每一个事件
func SetMonitor(f emitter.MonitorFunc) {
	emitter.Monitor.Register(f)
}
