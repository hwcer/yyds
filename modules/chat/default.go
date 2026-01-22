package chat

var Options = struct {
	ChatCap   int // 聊天缓冲区容量
	PageSize  int // 每页消息数量
	TextRune  int // 每条消息最大字符数, 0表示不限制,中文算1个字符
	TextBytes int // 每条消息最大字节数, 0表示不限制，按字节数判断，中文算3个字节
}{
	ChatCap:   5000,
	PageSize:  100,
	TextRune:  300,
	TextBytes: 0,
}
