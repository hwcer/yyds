package emitter

import "github.com/hwcer/updater"

// Events 全局事件,会持续触发每一个事件
var Events = events{}

type events []EventsFunc

type EventsFunc func(u *updater.Updater, name int32, vs ...int32)

func (es *events) Register(handle EventsFunc) {
	*es = append(*es, handle)
}

func (es *events) emit(u *updater.Updater, name int32, vs ...int32) {
	for _, handle := range *es {
		handle(u, name, vs...)
	}
}
