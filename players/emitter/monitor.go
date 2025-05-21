package emitter

import "github.com/hwcer/updater"

// Monitor 全局事件,会持续触发每一个事件
var Monitor = monitor{}

type monitor struct {
	handles []MonitorFunc
}

type MonitorFunc func(u *updater.Updater, name int32, vs ...int32)

func (es *monitor) Register(handle MonitorFunc) {
	es.handles = append(es.handles, handle)
}

func (es *monitor) emit(u *updater.Updater, name int32, vs ...int32) {
	for _, handle := range es.handles {
		handle(u, name, vs...)
	}
}
