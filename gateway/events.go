package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
)

var Emitter = emitter{events: make(map[EventType][]EventHandle)}

type EventType int8

type EventHandle func(player *session.Data, path string, meta values.Metadata)

const (
	EventTypeRequest EventType = iota //请求时
	EventTypeConfirm                  //确认消息
	EventTypeMessage                  //推送消息时
)

type emitter struct {
	events map[EventType][]EventHandle
}

func (e *emitter) emit(evt EventType, player *session.Data, path string, meta values.Metadata) {
	if handlers, ok := e.events[evt]; ok {
		for _, h := range handlers {
			h(player, path, meta)
		}
	}
}

func (e *emitter) Listen(evt EventType, h EventHandle) {
	e.events[evt] = append(e.events[evt], h)
}

func On(evt EventType, h EventHandle) {
	Emitter.Listen(evt, h)
}
func Listen(evt EventType, h EventHandle) {
	Emitter.Listen(evt, h)
}
