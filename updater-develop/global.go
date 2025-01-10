package updater

// 全局事件,会持续触发
var globalEvents = map[EventType][]func(u *Updater){}

// RegisterGlobalEvent 必须在初始化话时调用

func RegisterGlobalEvent(t EventType, handle func(u *Updater)) {
	globalEvents[t] = append(globalEvents[t], handle)
}
