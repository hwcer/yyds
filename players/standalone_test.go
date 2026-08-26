package players

import (
	"testing"

	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

// reset 还原本文件动过的三个全局量。它们是包级原子量,不还原会串到别的测试。
func reset(t *testing.T) {
	t.Cleanup(func() {
		playersStandalone.Store(false)
		playersMaintain.Store(false)
		playersState.Store(stateStopped)
	})
}

// TestStandaloneServiceable 没有玩家容器的服务能对外提供服务,但维护照常挡。
//
// 🔴 这条是「社交服直连」的地基。不声明 Standalone 时,这类进程的 playersState
// 恒为 stateStopped —— 而 context.handlerCaller 对任何外网路径的第一件事就是
// Serviceable(),且发生在读权限档位【之前】。于是它的每个对外接口都回
// ErrServerClosed,接口权限设成 None / OAuth / Select 都救不回来。
func TestStandaloneServiceable(t *testing.T) {
	reset(t)

	//未声明、未启动:照旧拒绝 —— 这是给有玩家容器的服务用的判定,不能被放宽
	if err := Serviceable(); err != errors.ErrServerClosed {
		t.Fatalf("未启动时应回 ErrServerClosed,实得 %v", err)
	}

	//声明之后可对外服务
	Standalone()
	if !IsStandalone() {
		t.Fatal("Standalone 之后 IsStandalone 应为 true")
	}
	if err := Serviceable(); err != nil {
		t.Fatalf("声明无玩家容器后应可对外服务,实得 %v", err)
	}

	//🔴 维护开关照常生效:维护的语义是"不对客户端提供服务",与有没有玩家容器无关。
	//漏掉这条的后果是维护期间这类服务仍在对外回数据。
	Maintain(true)
	if err := Serviceable(); err != errors.ErrServerMaintain {
		t.Fatalf("维护期间应回 ErrServerMaintain,实得 %v", err)
	}
	Maintain(false)
}

// TestStandaloneGetErrors 没有玩家容器时,要玩家数据的路径回明确错误而不是崩。
//
// ps 在这类进程里是 nil(从没调过 Start),不拦住就是空指针 panic。
// 触发路径是「把 Player 级的路由注册进了这类服务」——那种错编译期看不出来。
func TestStandaloneGetErrors(t *testing.T) {
	reset(t)
	Standalone()

	called := false
	err := Get("uid-1", func(p *player.Player) error {
		called = true
		return nil
	})
	if err != errors.ErrServerStandalone {
		t.Fatalf("应回 ErrServerStandalone,实得 %v", err)
	}
	if called {
		t.Fatal("回调不该被执行")
	}
}

// TestStandaloneDoesNotAffectNormal 声明只影响声明了的进程,不改变默认行为。
func TestStandaloneDoesNotAffectNormal(t *testing.T) {
	reset(t)
	playersState.Store(stateRunning)
	if err := Serviceable(); err != nil {
		t.Fatalf("正常启动后应可服务,实得 %v", err)
	}
	Maintain(true)
	if err := Serviceable(); err != errors.ErrServerMaintain {
		t.Fatalf("维护期间应回 ErrServerMaintain,实得 %v", err)
	}
}
