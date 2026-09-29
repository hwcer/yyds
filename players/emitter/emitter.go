package emitter

import (
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/errors"
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
	events map[int32][]*Context
	values map[int32][]emitterValues
}

func (e *Emitter) On(t int32, args []int32, handle Callback) (r *Context) {
	r = NewContext(args, handle, false)
	if e.events == nil {
		e.events = map[int32][]*Context{}
	}
	e.events[t] = append(e.events[t], r)
	return
}

// Eager 注册急切监听：Emit 调用返回前**当场执行**，不等提交期——适合请求进行中
// 就要同步结论的场合（如拦单：监听器内 u.Errorf 立刻反映到 Updater.Error）。
// ⚠️ 已执行的副作用不随请求回滚，监听器必须只做校验/只读判定。
func (e *Emitter) Eager(t int32, args []int32, handle Callback) (r *Context) {
	r = NewContext(args, handle, true)
	if e.events == nil {
		e.events = map[int32][]*Context{}
	}
	e.events[t] = append(e.events[t], r)
	return
}

// Emit 触发事件：急切监听（On 的 Eager 变体 / Events.Eager）**当场执行**；
// 惰性监听进缓冲，updater 提交期随事务派发（失败请求事件丢弃）。
// 什么时候收到反馈由**监听方注册时声明**，触发方无需关心。
func (e *Emitter) Emit(name int32, v int32, args ...int32) {
	vs := make([]int32, 0, len(args)+1)
	vs = append(vs, v)
	vs = append(vs, args...)
	e.doEvents(name, v, args, true)
	Events.dispatch(e.u, name, vs, true)
	if e.values == nil {
		e.values = map[int32][]emitterValues{}
	}
	e.values[name] = append(e.values[name], vs)
	Monitor.emit(e.u, name, vs...)
}

// Listen 监听事件,并比较args 如果成功,则回调handle更新val
//
// 可以通过 Emitter.Register 注册全局过滤器,默认参数一致通过比较
func (e *Emitter) Listen(name string, t int32, args []int32, handle Listener) (r *Context, err error) {
	if name == "" {
		return nil, errors.New("emitter: name must not be empty")
	}
	if e.events == nil {
		e.events = map[int32][]*Context{}
	}
	r = NewContextWithListener(name, args, handle, false)
	for i, l := range e.events[t] {
		if l.name == name {
			e.events[t][i] = r
			return r, nil
		}
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
			e.doEvents(et, v[0], v[1:], false)
			Events.dispatch(e.u, et, v, false)
		}
	}
	e.values = nil
	return true
}

func (e *Emitter) release(_ *updater.Updater) bool {
	e.values = nil
	return true
}

// doEvents 按 eagerOnly 派发本玩家监听（true=急切监听，false=惰性监听）。
// 另一时机的监听原样保留；本次执行的按 caller 返回值决定去留（false=移除）。
func (e *Emitter) doEvents(t int32, v int32, args []int32, eagerOnly bool) {
	if len(e.events[t]) == 0 {
		return
	}
	var dict []*Context
	for _, l := range e.events[t] {
		if l.eager != eagerOnly {
			dict = append(dict, l)
			continue
		}
		if l.caller(e.u, t, v, args) {
			dict = append(dict, l)
		}
	}
	e.events[t] = dict
}
