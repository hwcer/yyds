package chat

import "fmt"

// ChannelType 频道类型
type ChannelType int32

// 频道类型常量
const (
	ChannelTypeNone    ChannelType = 0 // 世界频道
	ChannelTypeUnion   ChannelType = 1 // 工会联盟频道
	ChannelTypePrivate ChannelType = 2 // 私聊频道
)

// Channel 频道信息
type Channel struct {
	Id     ChannelType `json:"id" bson:"id"`   // 频道类型
	Target string      `json:"tar" bson:"tar"` // 频道ID，私聊时为对方UID
}

// Message 聊天消息
type Message struct {
	Id      uint64         `json:"id"`                               // 唯一ID，每次服务器重启后重新计数
	Uid     string         `json:"uid" bson:"uid"`                   // 玩家UID
	Msg     string         `json:"msg" bson:"msg"`                   // 消息内容
	Args    map[string]any `json:"args" bson:"args"`                 // 附加参数，如玩家名称、等级、图标等
	Time    int64          `json:"time" bson:"time"`                 // 发布时间戳
	Channel *Channel       `json:"channel,omitempty" bson:"channel"` // 频道信息，默认全服
}

// Set 设置消息附加参数
// 参数：
//   k: 参数键名
//   v: 参数值，会被转换为字符串
func (this *Message) Set(k string, v any) {
	if this.Args == nil {
		this.Args = map[string]any{}
	}
	this.Args[k] = fmt.Sprintf("%v", v)
}
