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
	r = NewContext(args, handle)
	if e.events == nil {
		e.events = map[int32][]*Context{}
	}
	e.events[t] = append(e.events[t], r)
	return
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

// Dispatch 立即派发事件：同步执行已注册监听器（本玩家 Listen 与全局 Events），
// 不进提交期缓冲。
//
// 与 Emit 的区别：Emit 缓冲到 updater 提交期才派发（随事务成败同生共死，适合
// 发货回调这类"成功才算数"的场合）；Dispatch **当场执行**，适合请求进行中就要
// 同步结论的场合（如下单前校验拦单）——监听器内 u.Errorf 会立刻反映到
// Updater.Error，调用方随后检查即可拦下请求。
//
// ⚠️ Dispatch 已执行的副作用不随请求回滚（后续请求失败不会撤销），监听器
// 必须只做校验/只读判定，禁止在 Dispatch 监听器里写玩家数据。
func (e *Emitter) Dispatch(name int32, v int32, args ...int32) {
	vs := make([]int32, 0, len(args)+1)
	vs = append(vs, v)
	vs = append(vs, args...)
	e.doEvents(name, v, args)
	Events.Emit(e.u, name, vs...)
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
	r = NewContextWithListener(name, args, handle)
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
	var dict []*Context
	for _, l := range e.events[t] {
		if l.caller(e.u, t, v, args) {
			dict = append(dict, l)
		}
	}
	e.events[t] = dict
}
