package emitter

import (
	"github.com/hwcer/updater"
)

// listener 业务逻辑层面普通任务事件,返回false时将移除
type emitterValues []int32

func New(u *updater.Updater) *Emitter {
	i := &Emitter{u: u}
	u.Events.On(updater.EventTypeSubmit, i.emit)
	u.Events.On(updater.EventTypeRelease, i.release)
	return i
}

type Emitter struct {
	u      *updater.Updater
	events map[int32][]*Listener
	values map[int32][]emitterValues
}

func (e *Emitter) On(t int32, args []int32, handle Handle) (r *Listener) {
	return e.Listen(t, args, handle)
}

func (e *Emitter) Emit(name int32, v int32, args ...int32) {
	vs := make([]int32, 0, len(args)+1)
	vs = append(vs, v)
	vs = append(vs, args...)
	if e.values == nil {
		e.values = map[int32][]emitterValues{}
	}
	e.values[name] = append(e.values[name], vs)
	Monitor.emit(e.u, name, vs...)
}

// Listen 监听事件,并比较args 如果成功,则回调handle更新val
//
// 可以通过 Eemitter.Register 注册全局过滤器,默认参数一致通过比较
func (e *Emitter) Listen(t int32, args []int32, handle Handle) (r *Listener) {
	r = NewListener(args, handle)
	if e.events == nil {
		e.events = map[int32][]*Listener{}
	}
	e.events[t] = append(e.events[t], r)
	return
}

func (e *Emitter) emit(_ *updater.Updater) bool {
	if len(e.values) == 0 {
		return true
	}
	for et, vs := range e.values {
		for _, v := range vs {
			e.doEvents(et, v[0], v[1:])
			Events.Emit(e.u, et, v...)
		}
	}
	e.values = nil
	return true
}

func (e *Emitter) release(_ *updater.Updater) bool {
	e.values = nil
	return true
}

// doEvents
func (e *Emitter) doEvents(t int32, v int32, args []int32) {
	if len(e.events[t]) == 0 {
		return
	}
	var dict []*Listener
	for _, l := range e.events[t] {
		if l.Handle(t, v, args) {
			dict = append(dict, l)
		}
	}
	e.events[t] = dict
}
