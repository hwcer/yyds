package emitter

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
)

var started bool

func init() {
	cosgo.On(cosgo.EventTypStarted, func() error {
		started = true
		return nil
	})
}

// Events 全局事件,必须在init中初始化，禁止动态添加
var Events = events{}

type events map[int32][]EventsFunc
type EventsFunc func(u *updater.Updater, vs ...int32)

func (e events) On(t int32, handle EventsFunc) {
	e.Listen(t, handle)
}

func (e events) Listen(t int32, handle EventsFunc) {
	if started {
		logger.Alert("禁止在程序启动后动态添加全局事件")
	} else {
		e[t] = append(e[t], handle)
	}
	return
}

func (e events) Emit(u *updater.Updater, t int32, args ...int32) {
	if len(e[t]) == 0 {
		return
	}
	for _, l := range e[t] {
		l(u, args...)
	}
}
