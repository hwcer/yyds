package gomcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/gateway/gwcfg"
	yydscontext "github.com/hwcer/yyds/context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerCaller(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "handle_call",
		Description: "以指定 uid 的身份调用游戏服的 handle 接口(等价于该玩家发了一个协议),返回服务端回包。" +
			"path 用客户端路径如 /shop/getter;args 传 JSON 对象。可用路径见 routes_list。" +
			"注意:对离线玩家调用会把它载入内存并计入在线数,之后由空闲回收机制自然释放。",
	}, handleCall)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "handle_describe",
		Description: "查看某个 handle 接口的鉴权级别、是否 GM 接口、注册方式。构造 handle_call 参数前先看这里。",
	}, handleDescribe)
}

// ------------------------------------------------------------------ handle_call

type callArgs struct {
	Uid  string         `json:"uid" jsonschema:"玩家 uid;调用 OAuthTypeNone 级别接口时可留空"`
	Path string         `json:"path" jsonschema:"客户端接口路径,如 /shop/getter、/debug/add"`
	Args map[string]any `json:"args,omitempty" jsonschema:"接口入参,JSON 对象"`
	Guid string         `json:"guid,omitempty" jsonschema:"账号 guid;仅 /roles /create /select 等未选角接口需要"`
}

func handleCall(ctx context.Context, req *mcp.CallToolRequest, args callArgs) (*mcp.CallToolResult, any, error) {
	r, err := Call(args.Uid, args.Path, args.Args, &CallOptions{Guid: args.Guid})
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(r)
}

// ------------------------------------------------------------------ handle_describe

type describeArgs struct {
	Path string `json:"path" jsonschema:"客户端接口路径,如 /shop/getter"`
}

type describeReply struct {
	Path       string `json:"path"`
	Method     string `json:"method"`
	Registered bool   `json:"registered"`
	Permission string `json:"permission"`
	Master     bool   `json:"master"`
	Kind       string `json:"kind"`             //func / method / struct
	Binder     string `json:"binder,omitempty"` //注册该接口的对象类型
	Note       string `json:"note,omitempty"`
}

func handleDescribe(ctx context.Context, req *mcp.CallToolRequest, args describeArgs) (*mcp.CallToolResult, any, error) {
	path := strings.TrimSpace(args.Path)
	if path == "" {
		return nil, nil, fmt.Errorf("path 不能为空")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.ToLower(path)
	method := gwcfg.GetServiceMethod(path)
	per, full := gwcfg.Authorize.Get(gwcfg.ServiceTypeGame, path)

	r := &describeReply{
		Path:       path,
		Method:     method,
		Permission: permissionName[per],
		Master:     gwcfg.Authorize.IsMaster(full),
	}
	if r.Permission == "" {
		r.Permission = fmt.Sprintf("Unknown(%d)", per)
	}
	yydscontext.Service.Range(func(node *registry.Node) bool {
		//node.Name() 带 ServiceType 前缀(/game),换算成 serviceMethod 后再比
		if !strings.EqualFold(serviceMethod(node.Name()), method) {
			return true
		}
		r.Registered = true
		switch {
		case node.IsFunc():
			r.Kind = "func"
		case node.IsMethod():
			r.Kind = "method"
		default:
			r.Kind = "struct"
		}
		if b := node.Binder(); b != nil {
			r.Binder = fmt.Sprintf("%T", b)
		}
		return false
	})
	if !r.Registered {
		r.Note = "该路径未注册,先用 routes_list 确认正确路径"
	} else if r.Master {
		r.Note = "GM/内部接口,gomcp 以开发者身份调用可放行"
	}
	return jsonResult(r)
}
