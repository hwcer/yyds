// Package gomcp 在游戏服进程内嵌入一个 MCP(Model Context Protocol) server,
// 让 AI 代理可以直接观察运行中的服务器状态、以指定 uid 身份调用 handle 接口、
// 采集并分析 pprof 性能数据。
//
// 定位是开发调试工具:只在配置了 [mcp].address 时启动,线上不配置即完全不生效。
//
// 业务项目注册自己的工具直接用 SDK 的泛型函数:
//
//	mcp.AddTool(gomcp.Server, &mcp.Tool{Name: "config_query", ...}, handler)
//
// 配套的 Claude Code skill(把服务器地址注册进 AI 会话、或不注册直接调工具)
// 不必每个项目自己写,在项目根目录跑一次即可生成:
//
//	go run github.com/hwcer/yyds/gomcp/cmd/skill -appid <本项目appid>
//
// 生成的 connect.py 只用 Python 标准库,不需要虚拟环境;模板见 gomcp/skill。
package gomcp

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/yyds/options"
)

var m = &Module{}

func New() *Module {
	return m
}

type Module struct {
}

func (this *Module) Id() string {
	return ModuleName
}

func (this *Module) Init() (err error) {
	if err = options.Initialize(); err != nil {
		return
	}
	return cosgo.Config.UnmarshalKey(ModuleName, Config)
}

func (this *Module) Start() (err error) {
	return start()
}

func (this *Module) Close() (err error) {
	return stop()
}
