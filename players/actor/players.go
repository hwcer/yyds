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

// New 创建玩家容器。w 就是 actor 的**全局玩家通道**,模型说明见 syncer.go 顶部。
//
// cap / timeout 见 players.Options 的 LockerCap / LockerTimeout:
//
//	cap      通道排队深度。actor 下全服操作都排这一条队,不宜太小
//	timeout  排队超时。⚠ 它同时是**重入的唯一护栏**:通道内的代码再调一次 players.Get /
//	         Load(context.GetPlayer 就会这么干)时,新请求排在自己身后永远轮不到,
//	         靠这个超时退出并报错,而不是永久挂死
func New(cap int, timeout time.Duration) *Players {
	w = await.New(cap, timeout)
	return instance
}

type Players struct {
	player.Manage
}

// invoke 进入全局玩家通道执行 fn,**所有取得玩家的操作都要经过这里**
//
// 通道内再取一次 p.Lock(在 actor 下就是全局 gate,见 syncer.go):全局通道只保证
// "通道内彼此串行",挡不住 daemon / Terminate 那些直接调 Player.Lock 的协程,
// 两边落在同一把锁上才算互斥。
func invoke(p *player.Player, fn func() error) error {
	_, err := w.Call(func(any) (any, error) {
		p.Lock()
		defer p.Unlock()
		return nil, fn()
	}, nil)
	return err
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

// Locker 批量取得多个玩家,按调用方**在不在全局通道内**分流 —— self 就是这个信号
//
//	self != ""  调用方是玩家请求,已经在通道里了(请求都经 Get / Load 进来)。直接干活,
//	            🔴 **不能再进一次通道**:通道只有一个 worker,自己排在自己身后永远轮不到。
//	self == ""  调用方不在通道内:context.Mutex().Async 的 scc 协程、daemon、定时器。
//	            必须先排队进入,否则等于**完全没有同步**。
//
// 🔴 这个分流在 actor 下是**强制**的,不是优化:loading 已经不对目标做任何加锁
// (权限来自"人在通道里"),所以不进通道就意味着一把锁都没有 —— 与通道内的 worker、
// 与 daemon 的回收全部裸奔。
func (this *Players) Locker(self string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	if self == "" {
		return NewLockerWithLocker(uid, handle, args, done...)
	}
	return NewLocker(self, uid, args, handle, done...)
}
