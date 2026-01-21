package chat

// NotifyName 通知名称
// 用于在玩家对象中存储最后一条消息的ID
const NotifyName = "ChatNotifyIndex"

// Filter 消息过滤器
// 用于筛选符合条件的消息，返回 true 表示保留该消息
// 参数：
//
//	m: 要检查的消息
//
// 返回值：
//
//	true: 保留该消息
//	false: 过滤掉该消息
type Filter func(Message) bool

type Message interface {
	GetId() uint64
}

// Channel 频道信息
type Channel struct {
	Key string `json:"k" bson:"k"` // 频道类型
	Val string `json:"v" bson:"v"` // 频道ID，私聊时为对方UID
}

type Factory interface {
	New(id uint64, text string, args map[string]any, channel *Channel) Message //创建用户消息
}
