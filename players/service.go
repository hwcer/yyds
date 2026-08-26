package players

import (
	"sync/atomic"

	"github.com/hwcer/yyds/errors"
)

// 服务器可服务性:启动状态 + 维护开关
//
// 分工:启动状态由框架自己维护(Start 置位,daemon 的 shutdown 复位),业务层不要碰;
// 维护开关与创角开关由业务层置位(GM 指令、master 下发等)。
//
// 维护与创角的区别:维护不满足就整服不对外服务(Serviceable),创角开关只影响新角色创建,
// 由业务层在创角接口上自行判定,框架不检查。

const (
	stateStopped int32 = iota //未启动,或已进入关闭流程
	stateRunning              //正常提供服务
)

var (
	playersState      atomic.Int32 //启动状态,见 stateStopped / stateRunning
	playersMaintain   atomic.Bool  //维护中,业务层置位
	playersCreate     atomic.Bool  //是否开放创建新角色,业务层置位,默认关闭
	playersStandalone atomic.Bool  //本进程没有玩家容器,见 Standalone
)

// Standalone 声明**本进程没有玩家容器**,由这类服务在 Init 阶段调用。
//
// # 什么服务该调
//
// 只提供共享数据、不持有玩家存档的微服务(社交服的公会/好友、世界服的跨服榜单…)。
// 它们不调 Start,进程里根本没有 Players 容器 —— 那不是配置疏漏,而是刻意的物理保证:
// 「本服不碰玩家存档」这条约束靠"没有那个对象"来兑现,比靠纪律可靠。
//
// # 为什么需要这个声明
//
// Serviceable 原先只有一种解读:playersState != stateRunning 就是"服务没起来"。
// 对没有玩家容器的服务这个判定恒为真,于是它的**每一个外网接口都回 ErrServerClosed**,
// 而且是在读权限档位之前就被拒 —— 接口权限设成 None / OAuth / Select 都没用。
// 结果是这类服务只能提供内网接口,客户端要读它的数据就得绕一个游戏服转发,
// 而那一跳往往还持着玩家锁等一整个跨进程往返。
//
// 声明之后:
//
//	Serviceable  跳过启动状态判定,**维护开关照常生效**
//	Get / Load   返回明确错误而不是空指针崩溃(ps 为 nil)
//
// # 🔴 它不放宽任何权限
//
// 鉴权分级仍由网关决定。这类服务能提供的上限是 OAuthTypeSelect(要选角、给 uid,
// 但不进玩家协程);Player 级的路由走到 players.Get 会拿到 ErrServerStandalone,
// 因为那一档要的东西本进程确实没有。
func Standalone() {
	playersStandalone.Store(true)
}

// IsStandalone 本进程是否声明了「没有玩家容器」
func IsStandalone() bool {
	return playersStandalone.Load()
}

// Started 框架是否已启动且未进入关闭流程
func Started() bool {
	return playersState.Load() == stateRunning
}

// Maintain 维护开关:不传参只查询,传参则置位后返回新值
//
// 维护期间所有客户端请求一律被拒(ErrServerMaintain),但**内网 RPC 不受影响** ——
// 否则维护一旦打开就没法再关掉了。解除维护请走内部接口(RegisterPrivate 注册的那些)。
func Maintain(v ...bool) bool {
	if len(v) > 0 {
		playersMaintain.Store(v[0])
	}
	return playersMaintain.Load()
}

// Creatable 创角开关(即通常说的"是否开放注册"):不传参只查询,传参则置位后返回新值
//
// 与维护的区别:维护挡所有请求,这个只挡新角色创建,老角色照常登录玩。
// 框架本身不检查它 —— 创角是业务概念,由业务层在创角接口上自行判定;
// 这里只提供一个并发安全的开关,免得业务层各自造一套(原先走 options.Game.Values
// 那个普通 map,推送写与请求读并发访问会直接触发 Go 的并发读写 fatal)。
func Creatable(v ...bool) bool {
	if len(v) > 0 {
		playersCreate.Store(v[0])
	}
	return playersCreate.Load()
}

// available 玩家系统能否被访问,只判启动状态
//
// **不判维护**:维护的语义是"不对客户端提供服务",而 GM 接口、master 推送、调试工具
// 这些内部访问在维护期间恰恰是必须能用的 —— 维护往往就是为了做这些事。
// 维护闸门只设在客户端入口(context.handlerCaller),不要下沉到这里。
func available() error {
	//没有玩家容器的进程:ps 为 nil,继续往下走就是空指针崩溃。回明确错误,
	//让调用方(以及误注册成 Player 级的路由)拿到一句能看懂的话。
	if playersStandalone.Load() {
		return errors.ErrServerStandalone
	}
	if playersState.Load() != stateRunning {
		return errors.ErrServerClosed
	}
	return nil
}

// Serviceable 服务器当前能否对外提供服务,不能则返回对应错误码
//
// 接入层(context.handlerCaller)在处理任何客户端请求之前调用它,一律拒绝而不是
// 让请求半推半就地进到业务里 —— 关闭过程中放进来的请求会一边加载玩家、一边被
// shutdown 释放掉。
func Serviceable() error {
	//🔴 没有玩家容器的服务不判启动状态 —— 那个状态由 Start 置位,而它压根不调 Start。
	//判了的话它的每个外网接口都回 ErrServerClosed,且发生在读权限档位之前,
	//接口权限设成什么都救不回来。见 Standalone。
	//**维护开关照常生效**:维护的语义是"不对客户端提供服务",与有没有玩家容器无关。
	if !playersStandalone.Load() && playersState.Load() != stateRunning {
		return errors.ErrServerClosed
	}
	if playersMaintain.Load() {
		return errors.ErrServerMaintain
	}
	return nil
}
