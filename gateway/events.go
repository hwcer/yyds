package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosrpc/xshare"
)

var Emitter = emitter{events: make(map[EventType][]EventHandle)}

type EventType int8

type EventHandle func(player *session.Data, meta xshare.Metadata)

const (
	EventTypeRequest     EventType = iota //请求时
	EventTypePushMessage                  //推送消息时
)

type emitter struct {
	events map[EventType][]EventHandle
}

func (e *emitter) emit(evt EventType, player *session.Data, meta xshare.Metadata) {
	if handlers, ok := e.events[evt]; ok {
		for _, h := range handlers {
			h(player, meta)
		}
	}
}

func (e *emitter) Listen(evt EventType, h func(player *session.Data, meta xshare.Metadata)) {
	e.events[evt] = append(e.events[evt], h)
}
