package emitter

import (
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/condition"
)

// listener 业务逻辑层面普通任务事件,返回false时将移除
// 挂载在 player
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

func (e *Emitter) On(t int32, judge condition.Judge, handle Callback) (r *Context) {
	r = NewContext(judge, handle)
	if e.events == nil {
		e.events = map[int32][]*Context{}
	}
	e.events[t] = append(e.events[t], r)
	return
}

// Emit 触发事件：急切监听（注册时声明 eager=true 的）**当场执行**；
// 惰性监听进缓冲，updater 提交期随事务派发（失败请求事件丢弃）。
// 什么时候收到反馈由**监听方注册时声明**，触发方无需关心。
//
// 🔴 不能调包级 emitter.Emit：那是系统事件入口，会把惰性监听一并立即执行——
// 此处请求尚未过验证，提前跑惰性（可能写数据）破坏"随事务派发"的语义。
func (e *Emitter) Emit(name int32, v int32, args ...int32) {
	vs := make([]int32, 0, len(args)+1)
	vs = append(vs, v)
	vs = append(vs, args...)
	trigger(e.u, name, vs...)
	dispatch(e.u, name, vs, true)
	if e.values == nil {
		e.values = map[int32][]emitterValues{}
	}
	e.values[name] = append(e.values[name], vs)
}

// Listen 监听事件,同名覆盖参数和回调(提交期派发,时机语义见 Emit)。
// judge 非 nil 且非 JudgeNone 时,事件 args 过裁决,匹配计数作为 val 传给 handle
func (e *Emitter) Listen(name string, t int32, judge condition.Judge, handle Listener) (r *Context, err error) {
	if name == "" {
		return nil, errors.New("emitter: name must not be empty")
	}
	if e.events == nil {
		e.events = map[int32][]*Context{}
	}
	r = NewContextWithListener(name, judge, handle)
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
	//🔴 排空式派发:监听器回调里可能再 Emit(任务链),直接 range e.values 再置 nil 会把
	//嵌套事件非确定性丢弃(map 遍历碰巧走到才派发)。每轮先摘走再派发,新事件进下一轮,
	//直到没有新事件——同一次提交内任务链完整派发,行为确定。
	for len(e.values) > 0 {
		values := e.values
		e.values = nil
		for et, vs := range values {
			for _, v := range vs {
				dispatch(e.u, et, v, false)
				e.dispatch(et, v[0], v[1:])
			}
		}
	}
	return true
}

func (e *Emitter) release(_ *updater.Updater) bool {
	e.values = nil
	return true
}

// dispatch  派发本玩家监听。
// 另一时机的监听原样保留；本次执行的按 caller 返回值决定去留（false=移除）。
func (e *Emitter) dispatch(t int32, v int32, args []int32) {
	if len(e.events[t]) == 0 {
		return
	}
	var dict []*Context
	for _, l := range e.events[t] {
		if l.caller(e.u, v, args) {
			dict = append(dict, l)
		}
	}
	e.events[t] = dict
}
