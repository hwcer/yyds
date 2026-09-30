package emitter

import (
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/players/condition"
)

type Callback func(att values.Values, val int32) bool //满足条件后的更新器,返回false移除监听

type Listener interface {
	Listener(u *updater.Updater, att values.Values, val int32) bool
}

type Context struct {
	name     string          //可选去重
	judge    condition.Judge //裁决参数是否匹配
	listener Listener
	callback Callback
	Attach   values.Values
}

func NewContext(judge condition.Judge, callback Callback) *Context {
	return &Context{judge: judge, callback: callback, Attach: values.Values{}}
}
func NewContextWithListener(name string, judge condition.Judge, l Listener) *Context {
	return &Context{name: name, judge: judge, listener: l, Attach: values.Values{}}
}

func (l *Context) Judge() condition.Judge {
	return l.judge
}

func (l *Context) Name() string {
	return l.name
}

func (l *Context) caller(u *updater.Updater, v int32, args []int32) bool {
	if l.judge != nil && l.judge.GetJudge() != condition.JudgeNone {
		v = condition.JudgeCompare(l.judge, args...)
	}
	if l.callback != nil {
		return l.callback(l.Attach, v)
	} else if l.listener != nil {
		return l.listener.Listener(u, l.Attach, v)
	}
	return false // 无回调的监听无意义，移除
}
