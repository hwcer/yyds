package actor

import (
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

var instance = &Players{}

func init() {
	instance.Manage = *player.NewManage()
}

func New() *Players {
	w = await.New(10, time.Second*5)
	return instance
}

type Players struct {
	player.Manage
}

func invoke(p *player.Player, fn func() error) error {
	return p.Syncer.(*Syncer).invoke(fn)
}

// Get 只获取在线玩家，进入玩家通道执行
func (this *Players) Get(uid string, handle player.Handle) error {
	p, ok := this.Manage.Load(uid)
	if !ok {
		return errors.ErrNotOnline
	}
	return invoke(p, func() error {
		//状态判定必须在通道内、Reset 之前:released 会在通道内把 Updater 置空。
		//Updater==nil 是 Lock 留下的空壳(只占了锁位、没加载数据):它按定义就不是在线玩家,
		//这里必须挡住——否则会把 nil Updater 交给业务。返回 ErrNotOnline 也正好让
		//context/locker.go 与 gomcp 那两条「Get 失败→退回 Load」的降级链继续成立,
		//由 Load 的 Loading 去自愈它。
		if p.Denied(atomic.LoadInt32(&p.Status)) || p.Updater == nil {
			return errors.ErrNotOnline
		}
		p.Reset()
		defer p.Release()
		return handle(p)
	})
}

// Load 加载玩家并进入通道执行
func (this *Players) Load(uid string, test bool, handle player.Handle) error {
	r := newPlayer(uid, test)
	if i, loaded := this.Manage.LoadOrStore(r.Key(), r); loaded {
		r = i
		if r.Denied(atomic.LoadInt32(&r.Status)) {
			return errors.ErrLoginWaiting
		}
	}
	return invoke(r, func() error {
		if err := r.Loading(test); err != nil {
			this.Manage.Delete(r.Key())
			return err
		}
		r.Reset()
		defer r.Release()
		return handle(r)
	})
}

// Lock 独占该玩家但**不加载任何数据**,详见 players.Lock 的说明。
func (this *Players) Lock(uid string, handle player.Handle) error {
	r := newPlayer(uid, false)
	i, loaded := this.Manage.LoadOrStore(r.Key(), r)
	if loaded {
		r = i
	} else {
		//空壳留在 Manage 里,**不要**用完就删:处理期间别人(登录/Get/Load)可能已经
		//LoadOrStore 拿到同一个指针、正堵在通道外等,删掉等于让它在 Manage 之外操作
		//一个孤儿对象;打拒绝态更糟,会把一次合法登录直接拒了。
		//正确的归宿是自愈:谁需要数据谁调 Loading 把它补全(Load/Login 已经这么做),
		//没人来就由 daemon 正常回收(Reset/Destroy 已对空壳 nil-safe)。
		//
		//必须补一次心跳:player.New 不设 heartbeat(恒 0),而 daemon 按
		//Heartbeat()<offlineTime 判回收、内存压力下又按 heartbeat 升序先放谁——
		//不刷的话空壳是全场最先被释放的对象,等不到有人来自愈。preload 同款做法。
		r.KeepAlive(0)
	}
	return invoke(r, func() error {
		if loaded && r.Denied(atomic.LoadInt32(&r.Status)) {
			return errors.ErrNotOnline
		}
		//不调 Reset/Release,见 locker 版注释
		return handle(r)
	})
}

func (this *Players) Locker(self string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	return NewLocker(self, uid, args, handle, done...)
}
