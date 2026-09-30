package emitter

import (
	"github.com/hwcer/cosgo/phase"
	"github.com/hwcer/updater"
)

// triggers 触发所有事件时立即响应，可以提前做一些事情

var triggers []triggerFunc

type triggerFunc func(u *updater.Updater, name int32, vs ...int32)

// Register 注册全局触发器。与 Listen 同规：启动后禁止注册
func Register(handle triggerFunc) {
	if phase.Sealed() {
		phase.Alert("players/emitter.trigger.Register")
		return
	}
	triggers = append(triggers, handle)
}

func trigger(u *updater.Updater, name int32, vs ...int32) {
	for _, handle := range triggers {
		handle(u, name, vs...)
	}
}
