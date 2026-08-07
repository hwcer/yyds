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

// Load 加载玩家并进入通道执行;init=false 时只占锁位不加载数据,详见 players.Load
func (this *Players) Load(uid string, test, init bool, handle player.Handle) error {
	r := newPlayer(uid, test)
	i, loaded := this.Manage.LoadOrStore(r.Key(), r)
	if loaded {
		r = i
		if r.Denied() {
			return errors.ErrLoginWaiting
		}
	} else if !init {
		//空壳必须补一次心跳:player.New 不设 heartbeat(恒 0),而 daemon 按
		//Heartbeat()<offlineTime 判回收、内存压力下又按 heartbeat 升序先放谁——
		//不刷的话它是全场最先被释放的对象,等不到有人来自愈。preload 同款做法。
		r.KeepAlive(0)
	}
	return invoke(r, func() error {
		if !init {
			//只占锁位:不 Loading。空壳留在 Manage 里等人自愈,不删不打拒绝态——处理期间
			//别人可能已经拿到同一个指针正在等锁,删掉等于让它在 Manage 之外操作孤儿对象,
			//打拒绝态则会拒掉合法登录。
			if r.Denied() {
				return errors.ErrNotOnline
			}
			//Reset/Release 照做(两者对空壳是空操作):玩家本就在内存时,少了这一步会给出
			//一个"非 nil 但 now 是零值"的 Updater——能用、不报错、时间全错。
			//init=false 的语义是"不预加载数据",不是"对象处于半初始化状态"。
			r.Reset()
			defer r.Release()
			return handle(r)
		}
		if err := r.Loading(test); err != nil {
			this.Manage.Delete(r.Key())
			return err
		}
		r.Reset()
		defer r.Release()
		return handle(r)
	})
}

func (this *Players) Locker(self string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	return NewLocker(self, uid, args, handle, done...)
}
