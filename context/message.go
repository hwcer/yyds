package context

import (
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/updater/operator"
)

type Message struct {
	*values.Message
	Time   int64                `json:"time,omitempty"`
	Cache  []*operator.Operator `json:"cache,omitempty"`
	Dirty  []*operator.Operator `json:"dirty,omitempty"`
	Notify values.Byte          `json:"notify,omitempty"` //消息通知
}

func Error(err interface{}, args ...interface{}) *Message {
	msg := &Message{}
	msg.Message = values.Errorf(0, err, args...)
	return msg
}

func Errorf(code int32, err interface{}, args ...interface{}) *Message {
	msg := &Message{}
	msg.Message = values.Errorf(code, err, args...)
	return msg
}

func Parse(v interface{}) *Message {
	if r, ok := v.(*Message); ok {
		return r
	}
	msg := &Message{}
	msg.Message = values.Parse(v)
	return msg
}
