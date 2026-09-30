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

// On 注册全局事件监听。eager 传 true 为急切监听:随 Emit 当场执行;
// 默认(惰性)对系统事件(连接/断线等)当场执行、对业务事件(p.Emit)在 updater 提交期派发。
// 急切监听在请求未过验证时就可能运行,只做只读校验/拦单,禁止写玩家数据(副作用不随请求回滚)。
func On(t int32, handle emitter.EventsFunc, eager ...bool) {
	emitter.Listen(t, handle, eager...)
}

// Listen 注册全局事件监听,等同 On
func Listen(t int32, handle emitter.EventsFunc, eager ...bool) {
	emitter.Listen(t, handle, eager...)
}
