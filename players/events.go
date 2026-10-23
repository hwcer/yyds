package players

import (
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/players/inbox"
	"github.com/hwcer/yyds/players/player"
)

// player Event 管理的是用户生命周期
// updater Event 管理的是请求生命周期

const (
	EventConnect updater.EventType = -iota - 100
	EventReplace
	EventReconnect
	EventDisconnect
	EventOffline
)

// -100 段为框架保留（上面的玩家生命周期）；业务自定义系统事件在自定义段
//（如 -200）自行定义常量，机制（Emit/On/inbox.On）对任意 EventType 开放。

// 全局系统事件,
// 业务事件只走player不能走这里

// On 注册同步系统事件监听：随 Emit 立即执行（域级注册，永不取消）。
// 发射点可能持锁在请求路径上（Connect 等），监听只做轻量/只读工作；
// 重活与失败重试走 Async。
func On(t updater.EventType, handle updater.EventFunc) {
	updater.RegisterGlobalEvent(t, handle)
}

// Emit 触发全局系统事件：同步监听（On）立即执行；有异步监听（Inbox 注册）的事件
// 当场**无脑缓存**进收件箱（载荷只随异步缓存走，同步 EventFunc 无参），由 players
// daemon 心跳经执行池处理（见 players/inbox）。
//
// 🔴 调用方须持有玩家锁：同步路径读写 Updater 实例事件表、缓存路径写 Updater.Cache，
// 不持锁调用与请求协程构成数据竞争（框架内发射点 Connected/disconnect/offline 与
// 业务请求路径均天然持锁）。
func Emit(p *player.Player, t updater.EventType, args ...int32) {
	if p.Updater != nil {
		p.Updater.Emit(t)
	}
	if p.Inbox != nil {
		p.Inbox.Emit(t, args)
	}
}

// Inbox 注册异步（收件箱）系统事件监听，等同 inbox.On——事件注册入口统一在本包
//（On 同步 / Inbox 异步），实现于 players/inbox 子包。
func Inbox(t updater.EventType, handle inbox.Func) {
	inbox.On(t, handle)
}
