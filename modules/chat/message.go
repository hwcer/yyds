package chat

import (
	"fmt"
	"time"
)

// Message 聊天消息
type defaultMessage struct {
	Id      uint64         `json:"id"`                               // 唯一ID，每次服务器重启后重新计数
	Text    string         `json:"text" bson:"text"`                 // 消息内容
	Args    map[string]any `json:"args" bson:"args"`                 // 附加参数，如玩家名称、等级、图标等
	Time    int64          `json:"time" bson:"time"`                 // 发布时间戳
	Channel *Channel       `json:"channel,omitempty" bson:"channel"` // 频道信息，默认全服
}

// Set 设置消息附加参数
// 参数：
//
//	k: 参数键名
//	v: 参数值，会被转换为字符串
func (this *defaultMessage) Set(k string, v any) {
	if this.Args == nil {
		this.Args = map[string]any{}
	}
	this.Args[k] = fmt.Sprintf("%v", v)
}

// GetId 获取消息ID
func (this *defaultMessage) GetId() uint64 {
	return this.Id
}

type defaultFactory struct{}

// New 创建用户消息
func (this *defaultFactory) New(id uint64, text string, args map[string]any, channel *Channel) Message {
	return &defaultMessage{
		Id:      id,
		Text:    text,
		Args:    args,
		Time:    time.Now().Unix(),
		Channel: channel,
	}
}
