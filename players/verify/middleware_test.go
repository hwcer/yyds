package verify

import (
	"testing"

	"github.com/hwcer/updater"
)

// TestMiddlewareUnregisterOnRelease 中间件必须在 Release 阶段自我注销。
//
// Auto() 注册的中间件原先只在 EventTypeSubmit 返回 false。而 Submit 只在 handle
// 返回成功码时才会执行(yyds/context/service.go)，一旦请求在 Auto 之后、Submit 之前
// 中断(如扣道具不足直接置 u.Error)，中间件就注销不掉、残留到同一 Updater 的下一个
// 请求，把毫不相干的请求判成"条件未达成"。
//
// EventTypeRelease 每次请求结束必定触发(无论成功失败)，是唯一可靠的注销点。
func TestMiddlewareUnregisterOnRelease(t *testing.T) {
	m := &middleware{}
	if m.Emit(nil, updater.EventTypeRelease) {
		t.Fatal("EventTypeRelease 必须返回 false 把自己从中间件列表移除")
	}
	//请求过程中的其他事件不得提前注销，否则 Submit 时校验会被跳过
	for _, et := range []updater.EventType{
		updater.EventTypeInit,
		updater.EventTypeReset,
		updater.EventTypeData,
		updater.EventTypeVerify,
		updater.EventTypeSuccess,
	} {
		if !m.Emit(nil, et) {
			t.Fatalf("EventType(%d) 不应注销中间件", et)
		}
	}
}
