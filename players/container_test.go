package players

import (
	"testing"

	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/locker"
	"github.com/hwcer/yyds/players/player"
)

// 「本进程有没有玩家容器」由 ps 决定,不需要任何声明:ps 只在 Start 里被赋值,
// 所以 ps == nil ⇔ 从没启动过玩家系统。下面三条钉住由此派生的行为。

// reset 还原本文件动过的全局量。它们是包级状态,不还原会串到别的测试。
func reset(t *testing.T) {
	t.Cleanup(func() {
		ps = nil
		playersMaintain.Store(false)
		playersState.Store(stateStopped)
	})
}

// TestNoContainerServiceable 没有玩家容器的进程能对外提供服务,但维护照常挡。
//
// 🔴 这条是「社交服直连」的地基。ps 为 nil 时若仍按"玩家系统没起来"判定,
// 这类进程的**每个外网接口都回 ErrServerClosed** —— 而 context.handlerCaller
// 对任何外网路径的第一件事就是 Serviceable(),且发生在读权限档位【之前】,
// 于是接口权限设成 None / OAuth / Select 都救不回来。
func TestNoContainerServiceable(t *testing.T) {
	reset(t)
	ps = nil

	if err := Serviceable(); err != nil {
		t.Fatalf("没有玩家容器时应可对外服务,实得 %v", err)
	}

	//🔴 维护开关照常生效:维护的语义是"不对客户端提供服务",与有没有玩家容器无关
	Maintain(true)
	if err := Serviceable(); err != errors.ErrServerMaintain {
		t.Fatalf("维护期间应回 ErrServerMaintain,实得 %v", err)
	}
}

// TestHasContainerStillGated 有玩家容器时,启动状态照旧把关 —— 不能被放宽。
//
// 这是上一条的对照面:ps 非 nil(正常游戏服)而 playersState 还没翻到 running,
// 说明玩家系统未起来或正在关闭,必须拒绝。放宽的后果是关闭过程中放进来的请求
// 会一边加载玩家、一边被 shutdown 释放掉。
func TestHasContainerStillGated(t *testing.T) {
	reset(t)
	ps = locker.New(Options.LockerCap, Options.LockerTimeout)

	if err := Serviceable(); err != errors.ErrServerClosed {
		t.Fatalf("有容器但未启动时应回 ErrServerClosed,实得 %v", err)
	}
	playersState.Store(stateRunning)
	if err := Serviceable(); err != nil {
		t.Fatalf("启动后应可服务,实得 %v", err)
	}
}

// TestNoContainerGetErrors 没有玩家容器时,要玩家数据的路径回明确错误而不是崩。
//
// ps 为 nil,不拦住就是空指针 panic。触发路径是「把 Player 级的路由注册进了
// 这类服务」——那种错编译期看不出来,进程也照常启动。
func TestNoContainerGetErrors(t *testing.T) {
	reset(t)
	ps = nil

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
