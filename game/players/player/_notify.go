package player

import (
	"github.com/hwcer/cosgo/values"
)

type NotifyType int

const (
	NotifyTypeChat   NotifyType = 1 //聊天新消息
	NotifyTypeMail   NotifyType = 2 //新私人邮件
	NotifyTypeConfig NotifyType = 3 //平台config需要更新(公告，活动，全服邮件。。。)
)

type NotifyChat interface {
	Has() bool
}

type Notify struct {
	Mail bool
	Chat NotifyChat //聊天
}

// Get 获取最新通知
func (this *Notify) Get() (r values.Byte) {
	if this.Mail {
		r.Set(int(NotifyTypeMail))
	}
	if this.Chat != nil && this.Chat.Has() {
		r.Set(int(NotifyTypeChat))
	}
	return r
}

func (this *Notify) Set(t NotifyType, v any) {
	switch t {
	case NotifyTypeMail:
		this.Mail, _ = v.(bool)
	}
}
