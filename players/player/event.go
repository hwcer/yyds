package player

import "github.com/hwcer/cosgo/values"

const (
	EventConnect int8 = iota
	EventReplace
	EventReconnect
	EventDisconnect
)

var events = map[int8][]EventHandle{}

type EventHandle func(*Player, values.Metadata) error

func On(e int8, h EventHandle) {
	events[e] = append(events[e], h)
}

func Emit(e int8, p *Player, meta values.Metadata) error {
	for _, h := range events[e] {
		if err := h(p, meta); err != nil {
			return err
		}
	}
	return nil
}
