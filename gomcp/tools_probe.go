package gomcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/gateway/gwcfg"
	yydscontext "github.com/hwcer/yyds/context"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players"
	"github.com/hwcer/yyds/players/player"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerProbe(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "server_status",
		Description: "获取游戏服务器运行状态:服务器编号/名称/网关地址/开服时间/在线人数/内存玩家数/可服务性/进程信息。" +
			"online 是连线中(StatusConnected)人数,memory 是内存里的玩家对象总数(含掉线未释放的缓存)," +
			"service.serviceable 为 false 时客户端请求会被一律拒绝(见 reason)。排查任何问题的第一跳。",
	}, serverStatus)

	mcp.AddTool(s, &mcp.Tool{
		Name: "server_maintain",
		Description: "查询或切换服务器维护状态。不传 enable 只查询;传 true 进入维护、false 解除。" +
			"维护期间所有客户端请求一律被拒(错误码 server maintain),内网 RPC 与本调试服务不受影响。" +
			"这是会影响线上所有玩家的开关,确认清楚再传 enable。",
	}, serverMaintain)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "players_online",
		Description: "列出当前在线玩家(uid/状态/网关/登录与心跳时间)。",
	}, playersOnline)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "player_dump",
		Description: "查看单个玩家的基础信息;可选传 iids 读取指定道具/字段的数值。玩家不在线时从数据库只读加载。",
	}, playerDump)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "routes_list",
		Description: "列出游戏服所有已注册的 handle 接口路径及其鉴权级别,标注哪些是 GM(master)接口。调用 handle_call 前先查这里确认路径。",
	}, routesList)

	s.AddResource(&mcp.Resource{
		Name:        "routes",
		URI:         "yyds://routes",
		Description: "游戏服全部 handle 接口路径与鉴权级别",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		b, err := json.MarshalIndent(routes(), "", "  ")
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "application/json", Text: string(b)},
		}}, nil
	})
}

// ------------------------------------------------------------------ server_status

type statusArgs struct{}

type statusReply struct {
	Sid        int32          `json:"sid"`
	Name       string         `json:"name"`
	Local      string         `json:"local"`
	Address    string         `json:"address"`
	OpenTime   string         `json:"openTime"`
	ServerTime string         `json:"serverTime"`
	Online     int32          `json:"online"`
	Memory     int32          `json:"memory"`
	Service    map[string]any `json:"service"` //可服务性:started/maintain/serviceable
	Version    string         `json:"version"`
	Debug      bool           `json:"debug"`
	Pid        int            `json:"pid"`
	Goroutine  int            `json:"goroutine"`
	GoVersion  string         `json:"goVersion"`
}

func serverStatus(ctx context.Context, req *mcp.CallToolRequest, args statusArgs) (*mcp.CallToolResult, any, error) {
	r := &statusReply{
		Sid:        options.Game.Sid,
		Name:       options.Game.Name,
		Local:      options.Game.Local,
		Address:    options.Game.Address,
		OpenTime:   options.Game.Time,
		ServerTime: times.Format(),
		Online:     players.Online(),
		Memory:     players.Memory(),
		Service:    serviceState(),
		Version:    cosgo.Version,
		Debug:      cosgo.Debug(),
		Pid:        os.Getpid(),
		Goroutine:  runtime.NumGoroutine(),
		GoVersion:  runtime.Version(),
	}
	return jsonResult(r)
}

// ------------------------------------------------------------------ server_maintain

type maintainArgs struct {
	Enable *bool `json:"enable,omitempty" jsonschema:"true 进入维护,false 解除维护;不传则只查询当前状态"`
}

func serverMaintain(ctx context.Context, req *mcp.CallToolRequest, args maintainArgs) (*mcp.CallToolResult, any, error) {
	if args.Enable != nil {
		players.Maintain(*args.Enable)
	}
	return jsonResult(serviceState())
}

// serviceState 服务器可服务性快照,server_status 与 server_maintain 共用
func serviceState() map[string]any {
	r := map[string]any{
		"started":  players.Started(),
		"maintain": players.Maintained(),
	}
	if err := players.Serviceable(); err != nil {
		r["serviceable"] = false
		r["reason"] = err.Error()
	} else {
		r["serviceable"] = true
	}
	return r
}

// ------------------------------------------------------------------ players_online

type onlineArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"最多返回多少个玩家,默认 100"`
}

type onlineItem struct {
	Uid       string `json:"uid"`
	Guid      string `json:"guid"`
	Status    int32  `json:"status"`
	Connected bool   `json:"connected"`
	Gateway   uint64 `json:"gateway"`
	Login     string `json:"login"`
	Heartbeat string `json:"heartbeat"`
}

func playersOnline(ctx context.Context, req *mcp.CallToolRequest, args onlineArgs) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 100
	}
	//两段式:Range 只持有 Manage 读锁,不持玩家锁,回调里只能碰不可变字段(uid)。
	//Guid 要读 Updater 的 dataset,而业务协程正持锁改它,released() 又会在
	//Destroy(Updater=nil) 之后、ps.Delete 之前留下窗口(Delete 还被本次 Range 的读锁挡着,
	//窗口反而被撑大)。裸读轻则空指针 panic,重则撞上 dirty map 的
	//concurrent map read and write —— 那是 fatal error,recover 拦不住,直接打挂进程。
	//另外不能在 Range 内加玩家锁:持 Manage 读锁去抢玩家锁就是 worker() 刚拆掉的锁序倒置。
	var total int
	ps := make([]*player.Player, 0, limit)
	players.Range(func(_ string, p *player.Player) bool {
		total++
		if len(ps) < limit {
			ps = append(ps, p) //超出上限只继续计数
		}
		return true
	})
	//已脱离 Manage 读锁,逐个持玩家锁取详情。
	//这里直接用 p.Lock 而不是 players.Get:后者会走 Updater 的 Reset/Release 周期,
	//会 Emit 事件并推进 u.last(影响跨天重置判定),调试工具不该有这种副作用。
	r := make([]*onlineItem, 0, len(ps))
	for _, p := range ps {
		//锁外快筛:Locked 的锁被 Loading 持着(含 DB 读),Released 的锁被 Destroy 持着
		//(脏玩家含一次 BulkWrite),都可能几十毫秒。先跳过就不必排队等一把注定用不上的锁
		if p.Denied() {
			continue
		}
		p.Lock()
		//锁内复查:等锁期间 released() 可能已经跑完 Destroy,此时读到的是残值
		status := atomic.LoadInt32(&p.Status)
		if p.Denied(status) {
			p.Unlock()
			continue
		}
		r = append(r, &onlineItem{
			Uid:       p.Uid(),
			Guid:      p.Guid(),
			Status:    status,
			Connected: p.Connected(),
			Gateway:   p.Gateway,
			Login:     unix(p.Login),
			Heartbeat: unix(p.Heartbeat()),
		})
		p.Unlock()
	}
	return jsonResult(map[string]any{"total": total, "online": players.Online(), "players": r})
}

// ------------------------------------------------------------------ player_dump

type dumpArgs struct {
	Uid  string  `json:"uid" jsonschema:"玩家 uid"`
	Iids []int32 `json:"iids,omitempty" jsonschema:"可选,要读取的道具/字段 iid 列表"`
}

type dumpReply struct {
	Uid       string          `json:"uid"`
	Guid      string          `json:"guid"`
	Online    bool            `json:"online"`
	Status    int32           `json:"status"`
	Gateway   uint64          `json:"gateway"`
	Login     string          `json:"login"`
	Heartbeat string          `json:"heartbeat"`
	Values    map[int32]any   `json:"values,omitempty"`
	Counts    map[int32]int64 `json:"counts,omitempty"`
}

func playerDump(ctx context.Context, req *mcp.CallToolRequest, args dumpArgs) (*mcp.CallToolResult, any, error) {
	if args.Uid == "" {
		return nil, nil, fmt.Errorf("uid 不能为空")
	}
	r := &dumpReply{Uid: args.Uid}
	fill := func(p *player.Player) error {
		r.Guid = p.Guid()
		r.Status = atomic.LoadInt32(&p.Status)
		r.Gateway = p.Gateway
		r.Login = unix(p.Login)
		r.Heartbeat = unix(p.Heartbeat())
		if len(args.Iids) == 0 {
			return nil
		}
		//Select + Data 预加载非常驻数据,否则 Get 到的是空值
		keys := make([]any, 0, len(args.Iids))
		for _, iid := range args.Iids {
			keys = append(keys, iid)
		}
		p.Select(keys...)
		if err := p.Data(); err != nil {
			return err
		}
		r.Values = map[int32]any{}
		r.Counts = map[int32]int64{}
		for _, iid := range args.Iids {
			r.Values[iid] = p.Get(iid)
			r.Counts[iid] = p.Val(iid)
		}
		return nil
	}
	//先按在线玩家取;不在线时退回数据库只读加载
	if err := players.Get(args.Uid, func(p *player.Player) error {
		r.Online = true
		return fill(p)
	}); err != nil {
		r.Online = false
		if e := players.Load(args.Uid, fill); e != nil {
			return nil, nil, e
		}
	}
	return jsonResult(r)
}

// ------------------------------------------------------------------ routes_list

type routesArgs struct {
	Keyword string `json:"keyword,omitempty" jsonschema:"可选,只返回路径中包含该关键字的接口"`
}

type routeItem struct {
	Path       string `json:"path"`               //handle_call 用这个
	Method     string `json:"method"`             //完整 serviceMethod
	Permission string `json:"permission"`         //鉴权级别
	Master     bool   `json:"master,omitempty"`   //GM 接口(Authorize.SetMaster 标记)
	Internal   bool   `json:"internal,omitempty"` //内部接口(RegisterPrivate,客户端无法经网关访问)
}

func routesList(ctx context.Context, req *mcp.CallToolRequest, args routesArgs) (*mcp.CallToolResult, any, error) {
	r := routes()
	if args.Keyword != "" {
		k := strings.ToLower(args.Keyword)
		f := make([]*routeItem, 0, len(r))
		for _, v := range r {
			if strings.Contains(strings.ToLower(v.Path), k) {
				f = append(f, v)
			}
		}
		r = f
	}
	return jsonResult(map[string]any{"total": len(r), "routes": r})
}

var permissionName = map[gwcfg.OAuthType]string{
	gwcfg.OAuthTypeNone:   "None(无需登录)",
	gwcfg.OAuthTypeOAuth:  "OAuth(已登录未选角)",
	gwcfg.OAuthTypeSelect: "Select(已选角,不进玩家协程)",
	gwcfg.OAuthTypePlayer: "Player(已选角,进玩家协程)",
}

func routes() []*routeItem {
	r := make([]*routeItem, 0, 128)
	yydscontext.Service.Range(func(node *registry.Node) bool {
		method := serviceMethod(node.Name())
		path := clientPath(node.Name())
		per, full := gwcfg.Authorize.Get(options.ServiceTypeGame, path)
		name := permissionName[per]
		if name == "" {
			name = fmt.Sprintf("Unknown(%d)", per)
		}
		r = append(r, &routeItem{
			Path:       path,
			Method:     method,
			Internal:   !gwcfg.HasServiceMethod(method),
			Permission: name,
			Master:     gwcfg.Authorize.IsMaster(full),
		})
		return true
	})
	sort.Slice(r, func(i, j int) bool { return r[i].Path < r[j].Path })
	return r
}

// ------------------------------------------------------------------ 路径换算
//
// registry 里节点的全名形如 /game/handle/shop/getter,三段含义分别是:
//
//	/game    ServiceType,cosrpc 调用时作为 servicePath 单独传
//	/handle  网关前缀(gwcfg.Gateway.Prefix),标识这是外网接口
//	/shop/getter  客户端实际使用的路径
//
// 这里提供两个方向的换算,避免各处重复 TrimPrefix 出错。

// clientPath 全名 → 客户端路径(/shop/getter)
func clientPath(name string) string {
	p := serviceMethod(name)
	if gwcfg.HasServiceMethod(p) {
		p = gwcfg.TrimServiceMethod(p)
	}
	return slash(p)
}

// serviceMethod 全名 → cosrpc 的 serviceMethod(/handle/shop/getter)
//
// 注意 Service.Name() 返回的已经带前导斜杠("/game"),不要再自己拼。
func serviceMethod(name string) string {
	return slash(strings.TrimPrefix(name, slash(yydscontext.Service.Name())))
}

func slash(s string) string {
	if !strings.HasPrefix(s, "/") {
		return "/" + s
	}
	return s
}

// ------------------------------------------------------------------ 公共

func unix(v int64) string {
	if v <= 0 {
		return ""
	}
	return time.Unix(v, 0).Format(time.DateTime)
}

// jsonResult 结构体转 JSON 文本返回。第二个返回值(结构化输出)留空,
// 避免 SDK 为每个工具额外推导 outputSchema 导致工具列表膨胀。
func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return text(string(b)), nil, nil
}
