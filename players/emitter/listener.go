package emitter

import (
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/updater"
)

type Callback func(att values.Values, val int32) bool //满足条件后的更新器,返回false移除监听

type Listener interface {
	Listener(u *updater.Updater, att values.Values, val int32) bool
}

type Context struct {
	args     []int32 //任务匹配参数
	name     string  //可选去重
	listener Listener
	callback Callback
	Filter   FilterFunc //过滤函数
	Attach   values.Values
}

func NewContext(args []int32, callback Callback) *Context {
	return &Context{args: args, callback: callback, Attach: values.Values{}}
}
func NewContextWithListener(name string, args []int32, l Listener) *Context {
	return &Context{name: name, args: args, listener: l, Attach: values.Values{}}
}

func (l *Context) Args() (r []int32) {
	if n := len(l.args); n > 0 {
		r = make([]int32, n)
		copy(r, l.args)
	}
	return
}

func (l *Context) Name() string {
	return l.name
}

func (l *Context) caller(u *updater.Updater, t int32, v int32, args []int32) bool {
	if !l.compare(t, args) {
		return true
	}
	if l.callback != nil {
		return l.callback(l.Attach, v)
	} else if l.listener != nil {
		return l.listener.Listener(u, l.Attach, v)
	}
	return false // 无回调的监听无意义，移除
}

func (l *Context) compare(t int32, args []int32) bool {
	if len(l.args) == 0 {
		return true
	}
	if l.Filter != nil {
		return l.Filter(l.args, args)
	}
	return Filters.Compare(t, l.args, args)
}
