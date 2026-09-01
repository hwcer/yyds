package locker

import (
	"testing"
	"time"

	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

// testUid 造一个**合法**的 uid —— loading 会用 uuid.IsValid 挡非法值，
// 随手写个 "test-1" 会被挡在门外，测到的就不是我们想测的那条路了。
func testUid(index uint64) string {
	return uuid.NewUUID(1, 1, index).String(uuid.BaseSize)
}

// 本文件钉住一条契约：**批量锁只取得操作权限，不加载数据**。
//
// 它是静默失效的重灾区 —— 一旦有人在 loading 里把 Loading 加回去，
// 功能全部照常，只是每次跨玩家操作都多读六张表、还把离线玩家长期钉在内存里，
// 压测之前根本看不出来。所以用测试钉，不靠注释。

func init() {
	New(128, time.Second*5) //初始化 await worker 与 instance.Manage
}

// TestLockerShellForOfflinePlayer 玩家不在内存时给的是空壳：Updater 为 nil。
//
// 🔴 这是本次改动的核心断言。若这里拿到非 nil，说明 loading 又在加载数据了。
func TestLockerShellForOfflinePlayer(t *testing.T) {
	uid := testUid(1)
	var got *player.Player
	_, err := NewLocker([]string{uid}, nil, func(l player.Locker, _ any) (any, error) {
		got = l.Get(uid)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("批量锁不该因为玩家不在内存而失败,实得 %v", err)
	}
	if got == nil {
		t.Fatal("回调里应拿到玩家对象(空壳也是对象)")
	}
	if got.Updater != nil {
		t.Fatal("玩家不在内存时必须是空壳(Updater==nil);拿到非 nil 说明 loading 又在加载数据了")
	}
	//空壳要留在 Manage 里等人自愈,不能用完就删 —— 别人可能已经拿到同一个指针在等锁
	if _, ok := instance.Manage.Load(uid); !ok {
		t.Fatal("空壳必须留在 Manage 里(占锁位 + 等自愈)")
	}
	//心跳必须补:player.New 不设 heartbeat(恒 0),不刷的话它是 daemon 眼里全场最先被回收的
	if p, _ := instance.Manage.Load(uid); p.Heartbeat() == 0 {
		t.Fatal("空壳必须补一次心跳,否则会被 daemon 抢在自愈之前回收")
	}
}

// TestLockerAggregatesSkipShell 四个聚合方法遇到空壳一律跳过，不 panic。
//
// Submit 尤其要钉：它原本是裸 p.Updater.Submit()，空壳流过就是 nil panic，
// 而这条路只在「被操作者恰好离线」时才走到，测不出来就等着线上炸。
func TestLockerAggregatesSkipShell(t *testing.T) {
	uid := testUid(2)
	_, err := NewLocker([]string{uid}, nil, func(l player.Locker, _ any) (any, error) {
		l.Select("guild")
		if e := l.Data(); e != nil {
			return nil, e
		}
		if e := l.Verify(); e != nil {
			return nil, e
		}
		return nil, l.Submit()
	})
	if err != nil {
		t.Fatalf("空壳上跑聚合方法应全部安全跳过,实得 %v", err)
	}
}

// TestLockerReusesInMemoryPlayer 玩家在内存时用现成的，并且照常 Reset。
//
// Reset 不能省：少了它会给出一个"非 nil 但 now 是零值"的 Updater —— 能用、不报错、
// 时间基准是 1 年，是最难查的那类问题。
func TestLockerReusesInMemoryPlayer(t *testing.T) {
	uid := testUid(3)
	want := newPlayer(uid, false)
	want.Updater = updater.New(want) //New 出来的 now 是零值,只有 Reset 会设
	instance.Manage.LoadOrStore(want.Key(), want)

	var got *player.Player
	_, err := NewLocker([]string{uid}, nil, func(l player.Locker, _ any) (any, error) {
		got = l.Get(uid)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("批量锁失败:%v", err)
	}
	if got != want {
		t.Fatal("玩家已在内存时必须复用同一个对象,不能另造一个")
	}
	if got.Updater == nil {
		t.Fatal("在内存的玩家不该被当成空壳")
	}
	if got.Unix() == 0 {
		t.Fatal("Reset 没跑:会给出一个 now 为零值的 Updater,时间基准全错还不报错")
	}
}

// TestLockerRejectsInvalidUid 非法 uid 必须当场拒掉，不能在 Manage 里种空壳。
//
// 🔴 这条钉的是「不加载数据」带来的连带缺口：uid 合法性原先是 Player.Loading 的第一道
// 检查，Loading 一去掉就没人管了。漏掉的后果不是报错而是**静默** —— 任何字符串都能
// 种下一个空壳占着内存等回收，而调用方以为自己操作成功了。
func TestLockerRejectsInvalidUid(t *testing.T) {
	const bad = "not-a-valid-uid"
	_, err := NewLocker([]string{bad}, nil, func(l player.Locker, _ any) (any, error) {
		t.Fatal("非法 uid 不该进到回调里")
		return nil, nil
	})
	if err != errors.ErrArgEmpty {
		t.Fatalf("非法 uid 应回 ErrArgEmpty,实得 %v", err)
	}
	if _, ok := instance.Manage.Load(bad); ok {
		t.Fatal("非法 uid 不能在 Manage 里留下空壳")
	}
}

// TestLockerReleasesLock 回调返回后锁必须放掉，否则下一次批量锁永久挂死。
func TestLockerReleasesLock(t *testing.T) {
	uid := testUid(4)
	for i := 0; i < 2; i++ {
		if _, err := NewLocker([]string{uid}, nil, func(l player.Locker, _ any) (any, error) {
			return nil, nil
		}); err != nil {
			t.Fatalf("第 %d 次批量锁失败:%v", i+1, err)
		}
	}
}
