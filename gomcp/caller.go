package gomcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/gateway/gwcfg"
	yydscontext "github.com/hwcer/yyds/context"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players"
	"github.com/hwcer/yyds/players/player"
)

// resolve 把客户端路径解析成 cosrpc 的 serviceMethod。
//
// 游戏服有两类接口:
//
//	外网接口   context.Register        注册名带网关前缀,如 /handle/shop/getter
//	内部接口   context.RegisterPrivate 注册名不带前缀,如 /character/page(master 运营接口)
//
// 优先按外网接口解析,其次内部接口。两者都没有就直接报错——rpcx 对不存在的方法
// 会回一句误导性极强的 "can't find service game",不如在这里拦住并指路。
func resolve(path string) (string, error) {
	outer := gwcfg.GetServiceMethod(path)
	var hitOuter, hitInner bool
	yydscontext.Service.Range(func(node *registry.Node) bool {
		switch name := serviceMethod(node.Name()); {
		case strings.EqualFold(name, outer):
			hitOuter = true
		case strings.EqualFold(name, path):
			hitInner = true
		}
		return !(hitOuter && hitInner)
	})
	if hitOuter {
		return outer, nil
	}
	if hitInner {
		return path, nil
	}
	return "", fmt.Errorf("接口未注册:%s(已尝试 %s 与 %s),用 routes_list 查正确路径", path, outer, path)
}

// Gateway 是 gomcp 发起调用时使用的伪网关标识。
//
// 真实值由网关按连接分配(自增的 socket id),这里取一个极大值确保不会撞车——
// 撞车会让在线玩家被误判为同网关重连。
//
// 必须落在 int64 范围内:players.Connected 用 meta.GetInt64 读这个字段,而
// handlerCaller 用 meta.GetUint64 读,取值超过 int64 上限时两者解析结果不一致,
// 会导致每次调用都被判为顶号("Sign in elsewhere")。
const Gateway uint64 = 1 << 62

// prepare 确保玩家处于可被调用的状态,返回本次调用要用的 gateway 标识。
//
// yyds 的 handlerCaller 用 players.Get 取玩家,它只认「已在内存」的玩家,
// 对没登录过的直接回 "user not online";而 players.Connected 又要求 gateway 非零,
// 为零时回 "gateway is empty"。所以这里补两件事:
//
//	玩家已在内存 —— 沿用它当前的 gateway(为零则用 gomcp.Gateway),避免被判定顶号
//	玩家不在内存 —— 用 players.Login 载入并置为在线,之后由空闲回收机制自然释放
func prepare(uid string, gateway uint64) (uint64, error) {
	if uid == "" {
		return gateway, nil //无需玩家身份的接口
	}
	gate := gateway
	var loaded bool
	_ = players.Get(uid, func(p *player.Player) error {
		loaded = true
		if gate == 0 {
			gate = p.Gateway
		}
		return nil
	})
	if gate == 0 {
		gate = Gateway
	}
	if loaded {
		return gate, nil
	}
	meta := map[string]string{
		gwcfg.ServiceMetadataGateway: strconv.FormatUint(gate, 10),
		binder.HeaderAccept:          binder.Json.Name(),
		binder.HeaderContentType:     binder.Json.Name(),
	}
	if err := players.Login(uid, false, meta, func(*player.Player) error { return nil }); err != nil {
		return 0, fmt.Errorf("玩家 %s 载入失败(uid 是否存在?):%w", uid, err)
	}
	return gate, nil
}

// CallOptions 调用可选项。
type CallOptions struct {
	Guid    string //OAuthTypeOAuth 级别的接口(如 /roles /create /select)需要
	Gateway uint64 //网关标识,留空时自动取玩家当前网关,玩家离线则用 gomcp.Gateway
}

// Call 以指定 uid 的身份调用一个 handle 接口。
//
// 走的是和网关完全相同的内部 RPC 链路(client.CallWithMetadata → yyds/context.handlerCaller),
// 因此鉴权、玩家数据加载、updater 提交、回包序列化的行为与真实客户端请求一致,
// 区别只是 metadata 由本模块直接构造,不经过 cosnet 与网关转发。
//
// path 是客户端使用的外网路径(如 /shop/submit),内部会补上网关前缀。
func Call(uid, path string, args any, opts ...*CallOptions) (r map[string]any, err error) {
	opt := &CallOptions{}
	if len(opts) > 0 && opts[0] != nil {
		opt = opts[0]
	}
	if path = strings.TrimSpace(path); path == "" {
		return nil, fmt.Errorf("path 不能为空")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.ToLower(path)

	method, err := resolve(path)
	if err != nil {
		return nil, err
	}
	per, _ := gwcfg.Authorize.Get(options.ServiceTypeGame, path)

	gate, err := prepare(uid, opt.Gateway)
	if err != nil {
		return nil, err
	}

	req := map[string]string{
		gwcfg.ServiceMetadataUID:        uid,
		gwcfg.ServiceMetadataServerId:   strconv.Itoa(int(options.Game.Sid)),
		gwcfg.ServiceMetadataPermission: strconv.Itoa(int(per)),
		gwcfg.ServiceMetadataDeveloper:  "1", //开发工具身份,放行 master 级(/debug/*)接口
		binder.HeaderContentType:        binder.Json.Name(),
		binder.HeaderAccept:             binder.Json.Name(),
	}
	if opt.Guid != "" {
		req[gwcfg.ServiceMetadataGUID] = opt.Guid
	}
	if gate > 0 {
		req[gwcfg.ServiceMetadataGateway] = strconv.FormatUint(gate, 10)
	}

	var body []byte
	if args != nil {
		if body, err = json.Marshal(args); err != nil {
			return nil, err
		}
	}
	if body == nil {
		body = []byte("{}")
	}

	reply := make([]byte, 0)
	if err = client.CallWithMetadata(req, nil, options.ServiceTypeGame, method, body, &reply); err != nil {
		return nil, err
	}
	if len(reply) == 0 {
		return map[string]any{}, nil //无返回内容的接口(如 Noreply)
	}
	r = map[string]any{}
	if err = json.Unmarshal(reply, &r); err != nil {
		//回包不是 JSON 时原样带回,便于排查
		return map[string]any{"raw": string(reply)}, nil
	}
	return r, nil
}
