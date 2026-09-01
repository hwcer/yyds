package actor

import (
	"sync"
	"testing"
	"time"

	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

// 本文件钉住 actor 的模型:**全局玩家通道,排到 = 取得所有玩家的操作权限**。
//
// 前两条对应 2026-09-01 修掉的两个真 bug —— 当时 Syncer 是"每玩家一条 chan + 一个 worker",
// 两条都能实测复现,且都是永久性故障(panic / 永久挂起),不是概率问题。

func init() { New(128, time.Second*5) }

func testUid(index uint64) string {
	return uuid.NewUUID(1, 1, index).String(uuid.BaseSize)
}

// TestSyncerMutualExclusion 并发取得同一个玩家:互斥必须成立,且不能 panic。
//
// 🔴 旧实现在这里挂:Lock() 把 holding 通道写在共享字段上,后来者覆盖先来者,
// 实测 8 个协程同时"持有"同一个玩家,随后 close of closed channel。
func TestSyncerMutualExclusion(t *testing.T) {
	s := NewSyncer()
	var inside, maxInside int32
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if e := recover(); e != nil {
					t.Errorf("并发取得同一玩家不该 panic:%v", e)
				}
			}()
			for j := 0; j < 50; j++ {
				s.Lock()
				mu.Lock()
				inside++
				if inside > maxInside {
					maxInside = inside
				}
				mu.Unlock()
				time.Sleep(time.Microsecond)
				mu.Lock()
				inside--
				mu.Unlock()
				s.Unlock()
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("并发取得同一玩家卡死")
	}
	if maxInside > 1 {
		t.Fatalf("互斥失效:同时有 %d 个持有者", maxInside)
	}
}

// TestCrossOperateNoDeadlock 两个方向的跨玩家操作同时发生,不得死锁。
//
// 🔴 旧实现在这里挂:业务跑在玩家自己的 worker 上,「A 取 B」与「B 取 A」
// 各占着自己的 worker 等对方,无超时,永久挂起。
func TestCrossOperateNoDeadlock(t *testing.T) {
	done := make(chan struct{}, 2)
	run := func(uid string) {
		_ = invoke(newPlayer(uid, false), func() error {
			time.Sleep(50 * time.Millisecond) //留出窗口让对面也进来排队
			return nil
		})
		done <- struct{}{}
	}
	go run(testUid(20))
	go run(testUid(21))
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("双向跨玩家操作互等")
		}
	}
}

// TestLockerNoPerPlayerLock 这是 actor 存在的理由:一次取得,任意改。
//
// 批量取多个玩家不做任何逐个加锁 —— 权限来自"人在全局通道里"。所以既不会漏锁,
// 也不会因为取锁顺序成环;回调里拿到的就是这批人,可以随便改。
func TestLockerNoPerPlayerLock(t *testing.T) {
	uids := []string{testUid(1), testUid(2), testUid(3)}
	got := map[string]bool{}
	_, err := NewLockerWithLocker(uids, func(l player.Locker, _ any) (any, error) {
		l.Range(func(p *player.Player) bool {
			got[p.Uid()] = true
			return true
		})
		return nil, nil
	}, nil)
	if err != nil {
		t.Fatalf("批量取得玩家失败:%v", err)
	}
	for _, uid := range uids {
		if !got[uid] {
			t.Fatalf("回调里少了 %v", uid)
		}
	}
	//反向再取一次(顺序相反),不会因为取锁顺序死锁
	rev := []string{uids[2], uids[1], uids[0]}
	if _, err = NewLockerWithLocker(rev, func(l player.Locker, _ any) (any, error) {
		return nil, nil
	}, nil); err != nil {
		t.Fatalf("反序批量取得玩家失败:%v", err)
	}
}

// TestLockerShellAndInvalidUid 与 locker 模式同一份契约:
// 不在内存的玩家给空壳(Updater==nil),非法 uid 当场拒掉。
func TestLockerShellAndInvalidUid(t *testing.T) {
	uid := testUid(4)
	_, err := NewLockerWithLocker([]string{uid}, func(l player.Locker, _ any) (any, error) {
		p := l.Get(uid)
		if p == nil {
			return nil, errors.New("回调里应拿到玩家对象(空壳也是对象)")
		}
		if p.Updater != nil {
			return nil, errors.New("玩家不在内存时必须是空壳:拿到非 nil 说明又在加载数据了")
		}
		//四个聚合方法遇到空壳一律跳过,不 panic
		l.Select("guild")
		if e := l.Data(); e != nil {
			return nil, e
		}
		if e := l.Verify(); e != nil {
			return nil, e
		}
		return nil, l.Submit()
	}, nil)
	if err != nil {
		t.Fatalf("空壳路径应全部安全:%v", err)
	}

	if _, err = NewLockerWithLocker([]string{"not-a-valid-uid"}, func(l player.Locker, _ any) (any, error) {
		t.Fatal("非法 uid 不该进到回调里")
		return nil, nil
	}, nil); err != errors.ErrArgEmpty {
		t.Fatalf("非法 uid 应回 ErrArgEmpty,实得 %v", err)
	}
}

// TestLockerResetInMemory 玩家在内存时复用现成对象,并照常 Reset。
func TestLockerResetInMemory(t *testing.T) {
	uid := testUid(5)
	want := newPlayer(uid, false)
	want.Updater = updater.New(want) //New 出来的 now 是零值,只有 Reset 会设
	instance.Manage.LoadOrStore(want.Key(), want)

	_, err := NewLockerWithLocker([]string{uid}, func(l player.Locker, _ any) (any, error) {
		got := l.Get(uid)
		if got != want {
			return nil, errors.New("玩家已在内存时必须复用同一个对象")
		}
		if got.Unix() == 0 {
			return nil, errors.New("Reset 没跑:会给出一个 now 为零值的 Updater,时间基准全错还不报错")
		}
		return nil, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// TestAsyncEntersChannel context.Mutex().Async 那条路(self=="")必须真的进通道。
//
// 🔴 这条钉的是一个曾经存在的**安全回归**:loading 改成"不加锁"之后,Players.Locker
// 若还无条件走 NewLocker,self=="" 的调用方(Async 的 scc 协程 / daemon / 定时器)
// 就是一把锁都没有地在改玩家数据 —— 与通道内的 worker、与 daemon 的回收全部裸奔。
func TestAsyncEntersChannel(t *testing.T) {
	uids := []string{testUid(10), testUid(11)}
	var inside, maxInside int32
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//self 传空 = 模拟 Mutex().Async 的 scc 协程
			_, _ = instance.Locker("", uids, nil, func(l player.Locker, _ any) (any, error) {
				mu.Lock()
				inside++
				if inside > maxInside {
					maxInside = inside
				}
				mu.Unlock()
				time.Sleep(2 * time.Millisecond)
				mu.Lock()
				inside--
				mu.Unlock()
				return nil, nil
			})
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("self==\"\" 的批量取得卡死")
	}
	if maxInside > 1 {
		t.Fatalf("self==\"\" 没进通道:同时有 %d 个在改数据", maxInside)
	}
}
