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
	handles[id] = append(handles[id], hs...)
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

// TestRecordNoListener 无监听的事件零开销：不记录不置标记。
func TestRecordNoListener(t *testing.T) {
	u, a := newInbox("t1")
	a.Record(testEventA, []int32{1})
	if stateOf(u) != nil || a.Pending() {
		t.Fatal("无监听不应记录/置标记")
	}
}

// TestRecordPayload 记录带载荷原样送达，Flush 后标记清零。
func TestRecordPayload(t *testing.T) {
	var got []int32
	registerOn(t, testEventA, func(_ *updater.Updater, vs ...int32) int64 {
		got = append([]int32(nil), vs...)
		return 0
	})
	u, a := newInbox("t2")
	a.Record(testEventA, []int32{5, 6})
	s := stateOf(u)
	if s == nil || len(s.events) != 1 || s.events[0].t != testEventA {
		t.Fatalf("应当场记录,实得 %+v", s)
	}
	if !a.Pending() {
		t.Fatal("记录后应置标记(daemon 扫描发现)")
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
// 首次投递执行并写入门闸；门闸期内的新事件在 Record 处丢弃（不进列表不置标记）；
// 时间推进到门闸后【空转不执行】——到期后必须再收到事件才触发。
func TestGatedDropAndRefireOnNewEvent(t *testing.T) {
	var runs int
	registerOn(t, testEventA, func(*updater.Updater, ...int32) int64 {
		runs++
		return 2000 // now 固定 1000：执行后门闸内
	})
	u, a := newInbox("t3")
	a.Record(testEventA, []int32{1})
	a.flush() // 首次投递无门闸可判：执行并写入 nexts=2000
	if runs != 1 {
		t.Fatal("首次投递应执行并写入门闸")
	}
	a.Record(testEventA, []int32{2})
	a.flush()
	if runs != 1 {
		t.Fatal("CD 未到不应执行")
	}
	s := stateOf(u)
	if s == nil || len(s.events) != 0 || s.nexts[testEventA] != 2000 {
		t.Fatalf("门闸内的事件应丢弃且门闸已写入,实得 events=%d nexts=%v", len(s.events), s.nexts)
	}
	if a.Pending() {
		t.Fatal("丢弃不置标记,daemon 不做重活")
	}
	// 时间推进到门闸之后：无待处理记录，tick 空转不执行（无新事件不触发）
	u.Reset(time.Unix(2000, 0))
	a.flush()
	if runs != 1 {
		t.Fatalf("到期后无新事件不应执行,runs=%d", runs)
	}
	// 到期后再收到事件：触发
	a.Record(testEventA, []int32{3})
	a.flush()
	if runs != 2 {
		t.Fatalf("到期后新事件应执行,runs=%d", runs)
	}
}

// TestGatedBatchDropsTail 同批同事件多条：首条执行设新门闸后，
// 其余条目被同批新设的门闸盖住——直接丢弃（同批只跑首条，不逐窗补跑）。
func TestGatedBatchDropsTail(t *testing.T) {
	var runs int
	registerOn(t, testEventA, func(u *updater.Updater, _ ...int32) int64 {
		runs++
		return u.Unix() + 100 // 发射时刻+100：同批其余条目被盖住
	})
	u, a := newInbox("t4")
	for i := 0; i < 3; i++ {
		a.Record(testEventA, []int32{int32(i)})
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

// TestDuePeek 门闸窥探：门闸内到达的事件在 Record 处丢弃（无记录→Due=false，
// daemon 本 tick 跳过重活）；有待处理记录则 Due=true。
func TestDuePeek(t *testing.T) {
	registerOn(t, testEventA, func(u *updater.Updater, _ ...int32) int64 {
		return u.Unix() + 300 // 未来门闸
	})
	_, a := newInbox("t5")
	a.Record(testEventA, []int32{1})
	if !a.due() {
		t.Fatal("有待处理记录 Due 应为 true")
	}
	a.flush() // 执行并设未来门闸（1300 > now=1000）
	a.Record(testEventA, []int32{2})
	if a.due() {
		t.Fatal("门闸内事件 Record 即丢弃,Due 应为 false")
	}
	if a.Pending() {
		t.Fatal("丢弃不置标记")
	}
}

// TestCDStrictest 多个监听都设 CD 时取更严（大）的 T。
func TestCDStrictest(t *testing.T) {
	registerOn(t, testEventA,
		func(*updater.Updater, ...int32) int64 { return 1100 },
		func(*updater.Updater, ...int32) int64 { return 1500 },
	)
	u, a := newInbox("t6")
	a.Record(testEventA, []int32{1})
	a.flush()
	if at := stateOf(u).nexts[testEventA]; at != 1500 {
		t.Fatalf("CD 应取更严的 1500,实得 %d", at)
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
	a.Record(testEventA, []int32{1})
	a.flush() // 不应向上抛 panic
	if !called {
		t.Fatal("panic 的监听不应影响其余监听执行")
	}
	if at := stateOf(u).nexts[testEventA]; at != 0 {
		t.Fatalf("panic 应按无 CD 处理,实得 %d", at)
	}
}

// TestNestedChain 嵌套链：监听里再记录的新事件（生产经 players.Emit→Record）重新置位，
// 下轮 Flush 处理——任务链跨轮收敛。
func TestNestedChain(t *testing.T) {
	var order []int32
	var self *Inbox
	registerOn(t, testEventA, func(*updater.Updater, ...int32) int64 {
		order = append(order, 1)
		self.Record(testEventB, []int32{9}) // 嵌套：生产中经 players.Emit 发射后记录于此
		return 0
	})
	registerOn(t, testEventB, func(*updater.Updater, ...int32) int64 {
		order = append(order, 2)
		return 0
	})
	_, self = newInbox("t8")
	self.Record(testEventA, []int32{1})
	self.flush()
	if len(order) != 1 || order[0] != 1 {
		t.Fatalf("首轮只应执行 A,实得 %v", order)
	}
	if s := stateOf(self.u); len(s.events) != 1 || s.events[0].t != testEventB {
		t.Fatalf("嵌套事件 B 应已记录,实得 %+v", s.events)
	}
	self.flush()
	if len(order) != 2 || order[1] != 2 {
		t.Fatalf("下轮应执行嵌套事件 B,实得 %v", order)
	}
}

// TestTick 心跳入口：到期走全套（Reset→派发→尾随 Submit 后标记清零）；
// 未来门闸则本 tick 直接返回不执行。
func TestTick(t *testing.T) {
	var runs int
	registerOn(t, testEventA, func(*updater.Updater, ...int32) int64 {
		runs++
		return 0
	})
	u, a := newInbox("t9")
	a.Record(testEventA, []int32{1})
	a.Trigger()
	if runs != 1 || a.Pending() {
		t.Fatalf("到期应派发并清标记,runs=%d pending=%v", runs, a.Pending())
	}
	// 未来门闸：门闸内事件 Record 即丢弃（无记录→Tick 直接返回，不 Reset 不派发）
	registerOn(t, testEventB, func(u *updater.Updater, _ ...int32) int64 {
		return u.Unix() + 300
	})
	a.Record(testEventB, []int32{1})
	a.Trigger() // B 首派设未来门闸（u.Unix() 已被本次 Reset 刷新为墙钟）
	a.Record(testEventB, []int32{2})
	if a.Pending() {
		t.Fatal("门闸内事件应丢弃,不置标记")
	}
	before := u.Unix()
	a.Trigger()
	if u.Unix() != before {
		t.Fatal("无待处理 Tick 不应 Reset（轻量返回）")
	}
}
