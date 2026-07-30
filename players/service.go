package players

import (
	"sync/atomic"

	"github.com/hwcer/yyds/errors"
)

// 服务器可服务性:启动状态 + 维护开关
//
// 两者分工:启动状态由框架自己维护(Start 置位,daemon 的 shutdown 复位),业务层不要碰;
// 维护开关由业务层置位(GM 指令、master 下发等)。任一不满足就不对外提供服务。

const (
	stateStopped int32 = iota //未启动,或已进入关闭流程
	stateRunning              //正常提供服务
)

var (
	playersState    atomic.Int32 //启动状态,见 stateStopped / stateRunning
	playersMaintain atomic.Bool  //维护中,业务层置位
)

// Started 框架是否已启动且未进入关闭流程
func Started() bool {
	return playersState.Load() == stateRunning
}

// Maintain 置位/解除维护状态,由业务层调用
//
// 维护期间所有客户端请求一律被拒(ErrServerMaintain),但**内网 RPC 不受影响** ——
// 否则维护一旦打开就没法再关掉了。解除维护请走内部接口(RegisterPrivate 注册的那些)。
func Maintain(v bool) {
	playersMaintain.Store(v)
}

// Maintained 当前是否处于维护状态
func Maintained() bool {
	return playersMaintain.Load()
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
