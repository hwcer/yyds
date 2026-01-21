package chat

import (
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/yyds/players/player"
)

var Default = New(1024, nil)

// Send 发送聊天
//
// msg string 聊天内容 200 字节
func Send(text string, args map[string]any, channel *Channel) (Message, error) {

	if n := len(text); n == 0 || n > 300 {
		return nil, values.Error("msg empty or too long")
	}
	if utils.IncludeNotPrintableChar(text) {
		return nil, values.Error("非法字符")
	}

	m := Default.Write(text, args, channel)
	return m, nil
}

// Getter 获取最新聊天记录
//
// n 当前索引值,应当记录，现在获取时出入(i)
// []*chat.Message
func Getter(p *player.Player, size int, filter Filter) []Message {
	return Default.Getter(p, size, filter)
}

// Notify 获取是否有新消息
func Notify(p *player.Player) {
	Default.Notify(p)
}
