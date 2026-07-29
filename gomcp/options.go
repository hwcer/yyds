package gomcp

// ModuleName 模块名,同时也是配置段名([mcp])
const ModuleName = "mcp"

// Options [mcp] 配置段
//
// 只有 Address 非空时模块才会启动,线上默认不配置即完全不生效。
type Options struct {
	Address string //监听地址,留空不启动;默认绑 127.0.0.1 避免测试机误暴露
	Token   string //非空时要求 Authorization: Bearer <token>
	Dump    string //pprof 原始文件落盘目录,留空使用系统临时目录
}

var Config = &Options{}
