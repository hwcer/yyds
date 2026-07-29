package gomcp

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hwcer/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server 全局 MCP Server。
//
// 在包 init 阶段即创建,业务项目可在任意时刻(通常是自己的 init 或 Module.Init)
// 用 SDK 的泛型函数挂上项目专有工具:
//
//	mcp.AddTool(gomcp.Server, &mcp.Tool{Name: "config_query"}, handler)
var Server *mcp.Server

var httpServer *http.Server
var hooks []func(*mcp.Server)

// Register 登记一个工具注册函数,MCP 启用时才会被调用。
//
// 业务项目在自己的 init 里登记,不要直接 mcp.AddTool 到 Server:
// AddTool 在 schema 有问题时会 panic,直接注册会让「只是 import 了本包」的
// 游戏服起不来,而调试工具不该有这种杀伤力。
//
//	func init() {
//	    gomcp.Register(func(s *mcp.Server) {
//	        mcp.AddTool(s, &mcp.Tool{Name: "config_query"}, handler)
//	    })
//	}
func Register(f func(*mcp.Server)) {
	hooks = append(hooks, f)
}

func init() {
	Server = mcp.NewServer(&mcp.Implementation{
		Name:    "yyds-gomcp",
		Version: "1.0.0",
	}, nil)
}

func start() (err error) {
	if Config.Address == "" {
		return nil //未配置,不启动
	}
	//内置工具在这里注册而不是 init:mcp.AddTool 在 schema 有问题时会 panic,
	//放 init 里会让「只是 import 了本包」的游戏服直接起不来。调试工具不该有这种杀伤力。
	registerProbe(Server)
	registerCaller(Server)
	registerPprof(Server)
	for _, f := range hooks {
		f(Server) //业务项目通过 Register 登记的工具
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return Server
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	httpServer = &http.Server{
		Handler:           authorize(handler),
		ReadHeaderTimeout: 10 * time.Second,
	}
	//先同步 Listen 拿到端口,这样端口被占用时能在启动阶段立刻报错,
	//而不是丢进 goroutine 里静默失败(cosgo.pprofStart 用 Timeout 兜底,这里可以做得更准)。
	var ln net.Listener
	if ln, err = net.Listen("tcp", Config.Address); err != nil {
		return err
	}
	go func() {
		if e := httpServer.Serve(ln); e != nil && !errors.Is(e, http.ErrServerClosed) {
			logger.Alert("mcp server exit:%v", e)
		}
	}()
	logger.Trace("mcp server started success:%v", Config.Address)
	return nil
}

func stop() error {
	if httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return httpServer.Shutdown(ctx)
}

// authorize Bearer token 中间件,Config.Token 为空时不校验。
func authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if Config.Token != "" {
			v := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			//定长比较,避免计时侧信道
			if subtle.ConstantTimeCompare([]byte(v), []byte(Config.Token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// text 把文本包装成 MCP 工具返回值。
func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
