package emitter

import (
	"github.com/hwcer/cosgo/phase"
	"github.com/hwcer/updater"
)

type EventsFunc func(u *updater.Updater, vs ...int32)

var events = map[int32][]EventsFunc{}
var eagerness = map[int32][]EventsFunc{}

// On 注册惰性监听：经 Player.Emit 触发的事件在 updater 提交期才派发
func On(t int32, handle EventsFunc, eager ...bool) {
	Listen(t, handle, eager...)
}

// Listen 兼容旧名，等同 On。
func Listen(t int32, handle EventsFunc, eager ...bool) {
	if phase.Sealed() {
		phase.Alert("players/emitter.register(%d)", t)
		return
	}
	if len(eager) > 0 && eager[0] {
		if _, ok := eagerness[t]; !ok {
			eagerness[t] = []EventsFunc{}
		}
		eagerness[t] = append(eagerness[t], handle)
	} else {
		if _, ok := events[t]; !ok {
			events[t] = []EventsFunc{}
		}
		events[t] = append(events[t], handle)
	}
}

// Emit 系统事件入口（daemon/Terminate 的连接、断线、顶号等）：
// trigger + 急切/惰性监听**全部立即执行**。
//
// 这些事件发出时没有在途请求——不存在"等 updater 提交期"的那一天，不立即跑惰性监听
// 它就永远收不到（离线结算正是惰性监听，且断线时刻的写入经 bulkWrite 随 Destroy 落库，
// 没有回滚问题）。业务路径的派发不走这里：Player.Emit 只跑急切监听并缓冲惰性到提交期
// （见 Emitter.Emit），那时请求还没过验证，提前跑惰性会影响业务正确性。
func Emit(u *updater.Updater, t int32, args ...int32) {
	//没有数据的玩家(Load init=false 留下的空壳)不产生事件:框架侧虽然不解引用 u,
	//但业务注册的监听几乎一定会,把 nil 交出去等于让业务崩
	if u == nil {
		return
	}
	trigger(u, t, args...)
	dispatch(u, t, args, true)
	dispatch(u, t, args, false)
}

// dispatch 按 eager 标记派发（eagerOnly=true 跑急切监听，false 跑惰性监听）。
func dispatch(u *updater.Updater, t int32, args []int32, eager bool) {
	var es []EventsFunc
	if eager {
		es = eagerness[t]
	} else {
		es = events[t]
	}

	if len(es) == 0 {
		return
	}
	for _, fn := range es {
		fn(u, args...)
	}
}
