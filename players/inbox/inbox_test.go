package inbox

import (
	"testing"
	"time"

	"github.com/hwcer/updater"
)

// 测试用业务系统事件（-200 业务自定义段，避开框架 -100 保留段）。
const (
	testEventA updater.EventType = -iota - 200
	testEventB
)

type testEntity struct{ id string }

func (e *testEntity) Id() string { return e.id }

// newInbox 裸 Updater + 固定时刻 Reset：now 零值的 Unix() 是公元元年的负数，
// 会让 CD 门闸（u.Unix() < nexts）恒真误闸——生产路径 Reset 必先于一切，测试同构。
func newInbox(id string) (*updater.Updater, *Inbox) {
	u := updater.Default.New(&testEntity{id: id})
	u.Reset(time.Unix(1000, 0))
	return u, New(u)
}

// registerOn 白盒注册监听，用例结束自清理。
func registerOn(t *testing.T, id updater.EventType, hs ...Func) {
	t.Helper()
	old, exists := handles[id]
	for _, h := range hs {
		handles[id] = append(handles[id], listener{fn: h, idx: int32(len(handles[id]))})
	}
	t.Cleanup(func() {
		if exists {
			handles[id] = old
		} else {
			delete(handles, id)
		}
	})
}

func stateOf(u *updater.Updater) *state {
	s, _ := u.Cache.Get(cacheKey).(*state)
	return s
}

// TestEmitNoListener 无监听的事件零开销：不缓存不置标记。
func TestEmitNoListener(t *testing.T) {
	u, a := newInbox("t1")
	a.Emit(testEventA, []int32{1})
	if stateOf(u) != nil || a.Pending() {
		t.Fatal("无监听不应缓存/置标记")
	}
}

// TestEmitUnconditionalCache Emit 无脑缓存:门闸不拦缓存——监听闸着,事件照样入列置标记。
func TestEmitUnconditionalCache(t *testing.T) {
	registerOn(t, testEventA, func(*updater.Updater, ...int32) int64 {
		return 5000 //远期门闸
	})
	u, a := newInbox("t2")
	a.Emit(testEventA, []int32{1})
	a.flush() //执行并设门闸 5000,列表清空
	a.Emit(testEventA, []int32{2}) //闸内新事件
	if !a.Pending() {
		t.Fatal("门闸归属监听,Emit 不做门闸判断——闸内事件应照常置标记")
	}
	if s := stateOf(u); len(s.events) != 1 {
		t.Fatalf("闸内事件应入列待 flush 裁决,实得 %d 条", len(s.events))
	}
}

// TestEmitPayload 缓存带载荷原样送达,Flush 后标记清零。
func TestEmitPayload(t *testing.T) {
	var got []int32
	registerOn(t, testEventA, func(_ *updater.Updater, vs ...int32) int64 {
		got = append([]int32(nil), vs...)
		return 0
	})
	u, a := newInbox("t3")
	a.Emit(testEventA, []int32{5, 6})
	s := stateOf(u)
	if s == nil || len(s.events) != 1 || s.events[0].t != testEventA {
		t.Fatalf("应当场缓存,实得 %+v", s)
	}
	if !a.Pending() {
		t.Fatal("缓存后应置标记(daemon 扫描发现)")
	}
	a.flush()
	if len(got) != 2 || got[0] != 5 || got[1] != 6 {
		t.Fatalf("载荷应原样送达 [5 6],实得 %v", got)
	}
	if a.Pending() {
		t.Fatal("全排空后标记应清零")
	}
}

// TestGatedDropAndRefireOnNewEvent CD 门闸「闸住丢弃、新事件触发」：
// 首次投递执行并写入门闸；门闸期内的新事件照常缓存,但 flush 时对该监听丢弃；
// 时间推进到门闸后【空转不执行】——到期后必须再收到事件才触发。
func TestGatedDropAndRefireOnNewEvent(t *testing.T) {
	var runs int
	registerOn(t, testEventA, func(*updater.Updater, ...int32) int64 {
		runs++
		return 2000 // now 固定 1000：执行后门闸内
	})
	u, a := newInbox("t4")
	a.Emit(testEventA, []int32{1})
	a.flush() // 首次投递无门闸可判：执行并写入 nexts=2000
	if runs != 1 {
		t.Fatal("首次投递应执行并写入门闸")
	}
	a.Emit(testEventA, []int32{2}) // 闸内新事件:入列
	a.flush()                      // flush 对该监听丢弃该条,无脑清空列表
	if runs != 1 {
		t.Fatal("CD 未到不应执行")
	}
	if s := stateOf(u); s == nil || len(s.events) != 0 || s.nexts[gateKey{testEventA, 0}] != 2000 {
		t.Fatalf("闸内条目应被 flush 丢弃且门闸已写入,实得 events=%d nexts=%v", len(s.events), s.nexts)
	}
	if a.Pending() {
		t.Fatal("丢弃后列表已清空,标记应为零")
	}
	// 时间推进到门闸之后：无待处理记录,空转不执行（无新事件不触发）
	u.Reset(time.Unix(2000, 0))
	a.flush()
	if runs != 1 {
		t.Fatalf("到期后无新事件不应执行,runs=%d", runs)
	}
	// 到期后再收到事件：触发
	a.Emit(testEventA, []int32{3})
	a.flush()
	if runs != 2 {
		t.Fatalf("到期后新事件应执行,runs=%d", runs)
	}
}

// TestGatedBatchDropsTail 同批同事件多条：首条执行设新门闸后，
// 其余条目对该监听被同批新设的门闸盖住——直接丢弃（同批只跑首条，不逐窗补跑）。
func TestGatedBatchDropsTail(t *testing.T) {
	var runs int
	registerOn(t, testEventA, func(u *updater.Updater, _ ...int32) int64 {
		runs++
		return u.Unix() + 100 // 发射时刻+100：同批其余条目被盖住
	})
	u, a := newInbox("t5")
	for i := 0; i < 3; i++ {
		a.Emit(testEventA, []int32{int32(i)})
	}
	a.flush() // now=1000：首条执行（设门 1100），其余两条丢弃
	if runs != 1 || len(stateOf(u).events) != 0 {
		t.Fatalf("同批应只执行首条其余丢弃,runs=%d events=%d", runs, len(stateOf(u).events))
	}
	// 时间推进：无残留记录，空转不执行
	u.Reset(time.Unix(1100, 0))
	a.flush()
	if runs != 1 || a.Pending() {
		t.Fatalf("丢弃后无残留不应再执行,runs=%d pending=%v", runs, a.Pending())
	}
}

// TestPerListenerGate 🔴 门闸按事件×监听隔离:同事件三个监听——A 无 CD、B 短 CD、
// C 长门闸——B 闸着的窗口里 A 照常每次执行、C 的事件不丢给 A/B;各自 next 独立。
func TestPerListenerGate(t *testing.T) {
	var runA, runB, runC int
	registerOn(t, testEventA,
		func(*updater.Updater, ...int32) int64 { runA++; return 0 },
		func(*updater.Updater, ...int32) int64 { runB++; return 1050 },
		func(*updater.Updater, ...int32) int64 { runC++; return 5000 },
	)
	u, a := newInbox("t6")
	a.Emit(testEventA, []int32{1})
	a.flush() // 首轮三个都执行:nexts 各自 0/1050/5000
	if runA != 1 || runB != 1 || runC != 1 {
		t.Fatalf("首轮三监听都应执行 A=%d B=%d C=%d", runA, runB, runC)
	}
	// 门闸内再两轮事件:A(无CD)每轮执行,B/C 被各自门闸丢弃——互不干扰
	a.Emit(testEventA, []int32{2})
	a.flush()
	a.Emit(testEventA, []int32{3})
	a.flush()
	if runA != 3 || runB != 1 || runC != 1 {
		t.Fatalf("A 应每轮执行,B/C 应被各自门闸丢弃: A=%d B=%d C=%d", runA, runB, runC)
	}
	s := stateOf(u)
	if s.nexts[gateKey{testEventA, 1}] != 1050 || s.nexts[gateKey{testEventA, 2}] != 5000 {
		t.Fatalf("各监听 next 应独立,实得 %+v", s.nexts)
	}
	// B 门闸到期(1050),C 仍在闸内:新事件只带 B 不带 C
	u.Reset(time.Unix(1050, 0))
	a.Emit(testEventA, []int32{4})
	a.flush()
	if runA != 4 || runB != 2 || runC != 1 {
		t.Fatalf("B 到期应触发,C 应仍被闸: A=%d B=%d C=%d", runA, runB, runC)
	}
}

// TestPanicIsolation 单监听 panic 不影响其余监听，且按无 CD 处理。
func TestPanicIsolation(t *testing.T) {
	called := false
	registerOn(t, testEventA,
		func(*updater.Updater, ...int32) int64 { panic("boom") },
		func(*updater.Updater, ...int32) int64 { called = true; return 0 },
	)
	u, a := newInbox("t7")
	a.Emit(testEventA, []int32{1})
	a.flush() // 不应向上抛 panic
	if !called {
		t.Fatal("panic 的监听不应影响其余监听执行")
	}
	if at := stateOf(u).nexts[gateKey{testEventA, 0}]; at != 0 {
		t.Fatalf("panic 应按无 CD 处理,实得 %d", at)
	}
}

// TestNestedChain 嵌套链：监听里再缓存的新事件（生产经 players.Emit→Inbox.Emit）
// 重新置位，下轮 Flush 处理——任务链跨轮收敛。
func TestNestedChain(t *testing.T) {
	var order []int32
	var self *Inbox
	registerOn(t, testEventA, func(*updater.Updater, ...int32) int64 {
		order = append(order, 1)
		self.Emit(testEventB, []int32{9}) // 嵌套：生产中经 players.Emit 发射后缓存于此
		return 0
	})
	registerOn(t, testEventB, func(*updater.Updater, ...int32) int64 {
		order = append(order, 2)
		return 0
	})
	_, self = newInbox("t8")
	self.Emit(testEventA, []int32{1})
	self.flush()
	if len(order) != 1 || order[0] != 1 {
		t.Fatalf("首轮只应执行 A,实得 %v", order)
	}
	if s := stateOf(self.u); len(s.events) != 1 || s.events[0].t != testEventB {
		t.Fatalf("嵌套事件 B 应已缓存,实得 %+v", s.events)
	}
	self.flush()
	if len(order) != 2 || order[1] != 2 {
		t.Fatalf("下轮应执行嵌套事件 B,实得 %v", order)
	}
}

// TestTick 心跳入口：有待处理走全套（Reset→派发→尾随 Submit 后标记清零）；
// 无待处理则本 tick 直接返回,不 Reset（轻量路径）。
func TestTick(t *testing.T) {
	var runs int
	registerOn(t, testEventA, func(*updater.Updater, ...int32) int64 {
		runs++
		return 0
	})
	u, a := newInbox("t9")
	a.Emit(testEventA, []int32{1})
	a.Trigger()
	if runs != 1 || a.Pending() {
		t.Fatalf("应派发并清标记,runs=%d pending=%v", runs, a.Pending())
	}
	// 门闸监听:首派设未来门闸后,闸内事件被 flush 丢弃、列表清空——再无待处理
	registerOn(t, testEventB, func(u *updater.Updater, _ ...int32) int64 {
		return u.Unix() + 300
	})
	a.Emit(testEventB, []int32{1})
	a.Trigger() // B 首派设未来门闸（u.Unix() 已被本次 Reset 刷新为墙钟）
	a.Emit(testEventB, []int32{2})
	a.Trigger() // 闸内:flush 丢弃该条并清空列表
	if a.Pending() {
		t.Fatal("闸内条目被 flush 丢弃后,不应再有待处理")
	}
	before := u.Unix()
	a.Trigger()
	if u.Unix() != before {
		t.Fatal("无待处理 Tick 不应 Reset（轻量返回）")
	}
}
