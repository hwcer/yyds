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
	playersState    atomic.Int32 //启动状态,见 stateStopped / stateRunning
	playersMaintain atomic.Bool  //维护中,业务层置位
	playersCreate   atomic.Bool  //是否开放创建新角色,业务层置位,默认关闭
)

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
	if playersState.Load() != stateRunning {
		return errors.ErrServerClosed
	}
	if playersMaintain.Load() {
		return errors.ErrServerMaintain
	}
	return nil
}
