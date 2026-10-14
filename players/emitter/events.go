package emitter

import (
	"github.com/hwcer/cosgo/phase"
	"github.com/hwcer/updater"
)

// 全局业务事件
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
