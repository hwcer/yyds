package players

import (
	"testing"

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

// 本文件钉住一条契约：**批量锁兼容离线操作——每个玩家都有 Updater，不在内存的以懒加载建、不预拉**。
//
// 两个方向都是静默失效的重灾区：一旦有人在 loading 里换成整档 Loading，功能全部照常，
// 只是每次跨玩家操作都多读六张表、还把离线玩家长期钉在内存里；反过来退回空壳（Updater==nil），
// 业务就又得自己补数据。所以用测试钉，不靠注释。

type nopBulk struct{}

func (nopBulk) Update(any, any, ...any) {}
func (nopBulk) Insert(any, ...any)      {}
func (nopBulk) Delete(any, ...any)      {}
func (nopBulk) Submit() error           { return nil }
func (nopBulk) String() string          { return "nop" }

func init() {
	newManage() //初始化批量锁队列与管理器
	//updater.Loading 首载要求配了 BulkWrite(开服自检),懒加载也不例外
	updater.Config.BulkWrite = func(*updater.Updater) updater.BulkWrite { return nopBulk{} }
}

// TestLockerOfflineUpdaterForOfflinePlayer 玩家不在内存时以懒加载建 Updater：非 nil、未整档加载。
//
// 🔴 核心断言：Updater==nil 说明又退回了空壳；Loaded()==true 说明在整档加载数据。
func TestLockerOfflineUpdaterForOfflinePlayer(t *testing.T) {
	uid := testUid(1)
	var got *player.Player
	_, err := newBatch([]string{uid}, nil, func(l player.Locker, _ any) (any, error) {
		got = l.Get(uid)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("批量锁不该因为玩家不在内存而失败,实得 %v", err)
	}
	if got == nil || got.Updater == nil {
		t.Fatal("玩家不在内存时批量锁必须给出可用的 Updater(懒加载),不能是空壳")
	}
	if got.Updater.Loaded() {
		t.Fatal("懒加载不得整档加载数据(Loaded 应为 false)")
	}
	//留在 Manage 里等登录转正,不能用完就删 —— 别人可能已经拿到同一个指针在等锁
	if _, ok := manage.Load(uid); !ok {
		t.Fatal("懒加载玩家必须留在 Manage 里(占锁位 + 等转正)")
	}
	//心跳必须补:player.New 不设 heartbeat(恒 0),不刷的话它是 daemon 眼里全场最先被回收的
	if p, _ := manage.Load(uid); p.Heartbeat() == 0 {
		t.Fatal("必须补一次心跳,否则会被 daemon 抢在转正之前回收")
	}
	//Get 只交出整档在线的玩家:懒加载要走「Get 失败 → Load」降级链转正
	if err := get(uid, func(*player.Player) error { return nil }); err != errors.ErrNotOnline {
		t.Fatalf("Get 应把懒加载玩家视为不在线(ErrNotOnline),实得 %v", err)
	}
}

// TestLockerAggregatesOnOfflinePlayer 四个聚合方法对懒加载玩家照常工作，不 panic。
func TestLockerAggregatesOnOfflinePlayer(t *testing.T) {
	uid := testUid(2)
	_, err := newBatch([]string{uid}, nil, func(l player.Locker, _ any) (any, error) {
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
		t.Fatalf("懒加载玩家上跑聚合方法应全部成功,实得 %v", err)
	}
}

// TestLoadLazyMatchesLocker Load(lazy=true) 与批量锁同一套行为：不在内存的以懒加载建 Updater，
// 不再给 Updater==nil 的空壳；非法 uid 同样当场拒掉（acquire 共用）。
func TestLoadLazyMatchesLocker(t *testing.T) {
	uid := testUid(5)
	var got *player.Player
	if err := load(uid, false, true, func(p *player.Player) error { got = p; return nil }); err != nil {
		t.Fatalf("Load(lazy=true) 不该因为玩家不在内存而失败,实得 %v", err)
	}
	if got == nil || got.Updater == nil || got.Updater.Loaded() {
		t.Fatal("Load(lazy=true) 必须给出懒加载 Updater(非 nil、未整档),不能是空壳")
	}
	if err := load("not-a-valid-uid", false, true, func(*player.Player) error {
		t.Fatal("非法 uid 不该进到回调里")
		return nil
	}); err != errors.ErrArgEmpty {
		t.Fatalf("Load 非法 uid 应回 ErrArgEmpty,实得 %v", err)
	}
}

// TestLockerReusesInMemoryPlayer 玩家在内存时用现成的，并且照常 Reset。
//
// Reset 不能省：少了它会给出一个"非 nil 但 now 是零值"的 Updater —— 能用、不报错、
// 时间基准是 1 年，是最难查的那类问题。
func TestLockerReusesInMemoryPlayer(t *testing.T) {
	uid := testUid(3)
	want := player.New(uid, false)
	want.Updater = updater.Default.New(want) //New 出来的 now 是零值,只有 Reset 会设
	manage.LoadOrStore(want.Key(), want)

	var got *player.Player
	_, err := newBatch([]string{uid}, nil, func(l player.Locker, _ any) (any, error) {
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
	_, err := newBatch([]string{bad}, nil, func(l player.Locker, _ any) (any, error) {
		t.Fatal("非法 uid 不该进到回调里")
		return nil, nil
	})
	if err != errors.ErrArgEmpty {
		t.Fatalf("非法 uid 应回 ErrArgEmpty,实得 %v", err)
	}
	if _, ok := manage.Load(bad); ok {
		t.Fatal("非法 uid 不能在 Manage 里留下空壳")
	}
}

// TestLockerReleasesLock 回调返回后锁必须放掉，否则下一次批量锁永久挂死。
func TestLockerReleasesLock(t *testing.T) {
	uid := testUid(4)
	for i := 0; i < 2; i++ {
		if _, err := newBatch([]string{uid}, nil, func(l player.Locker, _ any) (any, error) {
			return nil, nil
		}); err != nil {
			t.Fatalf("第 %d 次批量锁失败:%v", i+1, err)
		}
	}
}
