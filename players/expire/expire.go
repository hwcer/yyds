// Package expire 自定义时间类型(times.ExpireType)注册表:业务在 init 期注册 Handle,
// player.Times 的 Start/Expire 对非内置段分发到这里。
//
// 段位分配:cosgo 内置 0-6,yyds 保留 8/9,业务自定义从 10 起(ExpireType 是 int8,
// 配置值超过 127 截断错位)。更多参考 times.ExpireType。
package expire

import (
	"github.com/hwcer/cosgo/phase"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/updater"
)

const (
	TimePlayerCreate times.ExpireType = 8 //角色创建日(yyds 保留段)
	TimeServerCreate times.ExpireType = 9 //开服时间(yyds 保留段)
)

// Handle 自定义日期校验。
//
// args 是配置行 t 之后的原始切片,约定 [开始,结束](与 player.Times.Start/Expire
// 同参):Start 语义取 args[0],Expire 语义取 args[1],均可能缺省或为 0,自行决定
// 语义——内置段的"v==0 → 1"归一化**不**作用于这里,handler 收到的就是原始值。
//
// u 是玩家 Updater,调用点持有玩家锁;可能为 nil(已释放对象的残留调用),
// 需要读玩家数据的实现请自行判空。
type Handle interface {
	Start(u *updater.Updater, t int64, args []int64) (r int64, err error)
	Expire(u *updater.Updater, t int64, args []int64) (r int64, err error)
}

var handles = map[times.ExpireType]Handle{}

func Get(t times.ExpireType) Handle {
	return handles[t]
}

// Register 注册自定义时间类型,由 player.Times 的 Start/Expire default 分支分发。
//
// 仅启动期(init)调用,phase 封板后拒绝——运行期注册与请求协程的读并发,
// 是 map 并发读写 fatal。同类型重复注册静默覆盖(与 condition.Register 同规)。
//
// ⚠ 类型取值必须避开:cosgo 内置段 0-6(times.ExpireType.Has)、yyds 保留的
// 8/9(TimePlayerCreate/TimeServerCreate),自定义段从 10 起。ExpireType 底层
// 是 int8,配置值超过 127 会截断错位撞到别的段。
func Register(t times.ExpireType, h Handle) {
	if phase.Sealed() {
		phase.Alert("players/expire.Register(%v)", t)
		return
	}
	handles[t] = h
}
