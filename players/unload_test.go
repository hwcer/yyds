package players

import (
	"sync/atomic"
	"testing"

	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/players/player"
)

// Player.Unload 仅供 daemon 回收:非 StatusReleased 一律拒绝、什么都不做。
// 业务层在正常状态下误调用,Updater 被抽走后本次 handler 与框架收尾当场 nil panic,
// 对象还留在 Manage 里成了在线的空壳——所以必须报错而不是照做。
func TestPlayerUnloadRequiresReleased(t *testing.T) {
	p := player.New(testUid(9), false)
	p.Updater = updater.Default.New(p)

	if err := p.Unload(); err == nil {
		t.Fatal("非 Released 状态 Unload 必须报错")
	}
	if p.Updater == nil {
		t.Fatal("被拒绝的 Unload 不得拆掉 Updater")
	}

	atomic.StoreInt32(&p.Status, player.StatusReleased)
	if err := p.Unload(); err != nil {
		t.Fatalf("Released 状态 Unload 应成功: %v", err)
	}
	if p.Updater != nil || p.Times != nil || p.Emitter != nil || p.Inbox != nil || p.Condition != nil {
		t.Fatal("Unload 后 Updater 与组件必须一起拆掉(createComponents 以 Times 判已建)")
	}
}
