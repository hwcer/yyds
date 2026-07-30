package context

import (
	"strconv"
	"time"

	"github.com/hwcer/cosrpc"
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/yyds/players/player"
)

type Context struct {
	*cosrpc.Context

	// Deprecated: 不要再使用,后续版本移除。
	//
	// 它在 handlerCaller 里是 players.Get 返回之后才执行的 —— 那时玩家锁已经释放、
	// Player 也已置 nil。闭包里若捕获了玩家对象再调 Updater 方法就是锁外访问:
	// 轻则读到 released 之后的残值,重则撞上 dataset 里 map 的并发读写,
	// 那是 recover 拦不住的 fatal error,直接打挂进程。
	// 而且它只在成功路径执行,handler 返回错误时根本不会被调用,当收尾钩子用并不可靠。
	// 需要请求后置逻辑请在 handler 内部 defer —— 那里锁还在,语义也完整。
	Next func()

	// Player 本次请求的玩家对象,**仅在 handler 执行期间有效**,此期间由 players.Get
	// 持有玩家锁;handler 返回后 handlerCaller 立即置 nil。
	// 不要存进闭包或全局跨请求使用:锁一释放,daemon 随时可能把这个对象释放掉。
	// 各方法的加锁要求见 player.Player 的类型注释。
	Player *player.Player
}

// Uid 角色ID
func (this *Context) Uid() string {
	if this.Player != nil {
		return this.Player.Uid()
	}
	if r := this.GetMetadata(gwcfg.ServiceMetadataUID); r != "" {
		return r
	}
	return ""
}

// GUid 账号ID
func (this *Context) GUid() string {
	if this.Player != nil {
		return this.Player.Guid()
	}
	if r := this.GetMetadata(gwcfg.ServiceMetadataGUID); r != "" {
		return r
	}
	return ""
}

// Now 当前时间
//
// 注意两种上下文的语义不同:有 Player 时返回的是 **本次请求的开始时间**
// (Updater.now,由 players.Get 里的 Reset 设定),一次请求内多次调用结果一致,
// 适合做同一批数据的时间戳;没有 Player 时(OAuthTypeNone / OAuthTypeOAuth 这类
// 未选角的接口)只能退化成墙钟,每次调用都不同。
// 需要"整个请求用同一个时刻"时务必确认自己在有 Player 的路径上。
// Unix / Milli 同理。
func (this *Context) Now() time.Time {
	if this.Player != nil {
		return this.Player.Now()
	}
	return time.Now()
}

// Unix 秒级时间戳,语义见 Now
func (this *Context) Unix() int64 {
	if this.Player != nil {
		return this.Player.Unix()
	}
	return time.Now().Unix()
}

// Milli 毫秒级时间戳,语义见 Now
func (this *Context) Milli() int64 {
	if this.Player != nil {
		return this.Player.Milli()
	}
	return time.Now().UnixMilli()
}

func (this *Context) Permission() gwcfg.OAuthType {
	auth := this.GetMetadata(gwcfg.ServiceMetadataPermission)
	if auth == "" {
		return gwcfg.OAuthTypeNone
	}
	l, err := strconv.Atoi(auth)
	if err != nil {
		return gwcfg.OAuthTypeNone
	}
	return gwcfg.OAuthType(l)
}
