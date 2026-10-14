package players

import (
	"testing"

	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/players/inbox"
)

// 测试用业务系统事件（-200 业务自定义段）。
const testEventEmit updater.EventType = -250

// TestEmitSyncAndAsyncCoexist players.Emit 集成：同步监听（RegisterGlobalEvent）立即执行；
// 收件监听（inbox.On）当场投递进 Inbox 并置标记（载荷只随记录走）。
// 注册表无反注册：测试用专属事件 id（-250 段），不与其他用例串扰。
func TestEmitSyncAndAsyncCoexist(t *testing.T) {
	syncCalled := false
	updater.RegisterGlobalEvent(testEventEmit, func(u *updater.Updater) { syncCalled = true })

	var got []int32
	inbox.On(testEventEmit, func(_ *updater.Updater, vs ...int32) int64 {
		got = append([]int32(nil), vs...)
		return 0
	})

	p := newInboxPlayer("te1")
	Emit(p, testEventEmit, 5, 6)
	if !syncCalled {
		t.Fatal("同步监听应随 Emit 立即执行")
	}
	if !p.Inbox.Pending() {
		t.Fatal("有异步监听应记录并置标记")
	}
	p.Inbox.Trigger()
	if len(got) != 2 || got[0] != 5 || got[1] != 6 {
		t.Fatalf("载荷应只随异步记录走 [5 6],实得 %v", got)
	}

	// 无异步监听的事件：只走同步,不记录
	Emit(p, -251, 1)
	if p.Inbox.Pending() {
		t.Fatal("无异步监听不应置标记")
	}
}
