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

// Eager 注册急切全局监听：随 Player.Emit 当场执行（不等提交期）。
// 只做校验/只读判定（如拦单）；副作用不随请求回滚，禁止写玩家数据。
func Eager(t int32, handle emitter.EventsFunc) {
	emitter.Events.Eager(t, handle)
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
