package player

import "github.com/hwcer/updater"

const (
	EventConnect updater.EventType = iota + 100
	EventReplace
	EventReconnect
	EventDisconnect
)
