package actor

import (
	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

var w *await.Await

type Args struct {
	uid    []string
	args   any
	handle player.LockerHandle
}

// NewLocker 批量取得多个玩家,**调用方已在全局玩家通道内**(业务请求都是)
//
// 既然已经在通道里,操作权限就已经到手了(排到 = 拿到所有玩家的操作权限),这里不再取任何锁,
// 只是把这批玩家找出来、Reset 好交给 handle。self 参数保留是为了与 locker 模式同签名,
// actor 下不参与任何判断。
//
// 🔴 **不要在这里重新进一次通道**:通道只有一个 worker,自己排在自己身后永远轮不到,
// 只会白等一个 LockerTimeout。
func NewLocker(self string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	l := &Locker{self: self, done: done}
	for _, v := range uid {
		if err := l.loading(v); err != nil {
			l.release()
			return nil, err
		}
	}
	defer l.release()
	return handle(l, args)
}

// NewLockerWithLocker 同上,但供**不在全局通道内**的调用方使用(daemon / 定时器 / 独立协程)
//
// 它比 NewLocker 多一步:先排队进入全局通道。进去之后两者做的是同一件事。
func NewLockerWithLocker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error) {
	msg := &Args{uid: uid, handle: handle, args: args}
	l := &Locker{done: done}
	return w.Call(func(a any) (any, error) {
		gate.Lock()
		defer gate.Unlock()
		return l.call(a)
	}, msg)
}

type Locker struct {
	self string
	dict map[string]*player.Player
	done []func()
}

// call 在全局 await 的协程里执行,this.self 恒为空(调用方不在任何玩家通道内)
func (this *Locker) call(args any) (reply any, err error) {
	msg, _ := args.(*Args)
	//🔴 defer 必须在取目标的循环**之前**注册:原先放在循环之后,loading 中途失败时直接
	//return,前面已经进入并占住的那几个通道**永远不会 Unlock** —— 通道无超时,后面排队的人
	//从此永远排不到,那几个玩家彻底卡死。NewLocker 那条路是显式 l.release() 处理的,这里漏了。
	defer this.release()
	for _, v := range msg.uid {
		if err = this.loading(v); err != nil {
			return
		}
	}
	return msg.handle(this, msg.args)
}

// release 收尾。**没有锁要放** —— 操作权限来自"人在全局通道里",
// 由进入通道的那一层(invoke / NewLockerWithLocker)在离开时统一交还。
func (this *Locker) release() {
	for _, p := range this.dict {
		p.Release()
	}
	for _, d := range this.done {
		d()
	}
	this.dict = nil
}

// loading 取得 uid 的操作权限。
//
// 🔴 **只占锁位,不加载数据**,与 Players.Load(init=false)、locker 版 loading 三处同款。
// 批量锁的职责是"取得操作权限",不是"把对方的存档拉起来"——Loading 会整表拉起该玩家
// 全部常驻模型,而批量锁的典型用途(改对方一两个字段、发条消息)一个字节都用不上。
// 需要数据的调用方按需自己加载,契约见 player.Locker 接口上的说明。
//
// 🔴 玩家不在内存时必须 **LoadOrStore 把空壳放进 Manage**,不能像原先那样只 Load
// 一下拿个游离对象:那样 r.Lock() 锁的是**别人看不见的对象**,并发的登录会
// LoadOrStore 出另一个实例各自加载 —— 占位锁一点互斥都不提供。
// 原先这条被"反正 Loading 会把数据拉起来"掩盖着,改成占位锁之后,互斥就是它的全部价值。
func (this *Locker) loading(uid string) (err error) {
	if this.dict == nil {
		this.dict = map[string]*player.Player{}
	}
	if _, ok := this.dict[uid]; ok {
		return nil
	}
	//🔴 uid 合法性必须在这里挡:原先它是 Player.Loading 的第一道检查,不加载数据就没人管了。
	//漏掉的后果不是报错而是**静默**——任何一个字符串都会在 Manage 里种下一个空壳,
	//占着内存等 daemon 回收,而调用方以为自己操作成功了。
	if !uuid.IsValid(uid) {
		return errors.ErrArgEmpty
	}
	r := newPlayer(uid, false)
	if i, loaded := instance.Manage.LoadOrStore(r.Key(), r); loaded {
		r = i
		//已在内存的也要判拒绝态:Released 的对象 Updater 已被销毁
		if r.Denied() {
			return errors.ErrNotOnline
		}
	} else {
		//空壳必须补一次心跳:player.New 不设 heartbeat(恒 0),而 daemon 按
		//Heartbeat()<offlineTime 判回收、内存压力下又按 heartbeat 升序决定先放谁——
		//不刷的话它是全场最先被释放的对象,等不到有人来自愈。与 Players.Load 同款。
		r.KeepAlive(0)
	}

	//🔴 **不对目标做任何加锁**:我们已经在全局玩家通道里,而"排到通道"就等于取得了所有
	//玩家的操作权限(见 syncer.go 顶部)。再逐个上锁既是多余的,也正是旧实现出 bug 的地方。
	//
	//Reset 照做(对空壳是空操作):玩家本就在内存时少了这一步,会给出一个"非 nil 但 now
	//是零值"的 Updater——能用、不报错、时间全错。
	r.Reset()
	this.dict[uid] = r
	return
}

// Select 标记待拉取的 key。
//
// ⚠ 空壳(Updater==nil,玩家不在内存)被跳过:它连 Updater 都没有,谈不上"标记待拉取"。
// 要对这类玩家走 Updater,先 p.Initialize() —— 但那会拉起他的全部常驻数据,
// 掂量清楚再用,见 player.Locker 的契约。
func (this *Locker) Select(keys ...any) {
	for _, p := range this.dict {
		if p.Updater == nil {
			continue
		}
		p.Select(keys...)
	}
}

// Data 拉取 Select 标记的数据,空壳跳过,理由同 Select
func (this *Locker) Data() error {
	for _, p := range this.dict {
		if p.Updater == nil {
			continue
		}
		if err := p.Data(); err != nil {
			return err
		}
	}
	return nil
}

func (this *Locker) Get(uid string) *player.Player {
	return this.dict[uid]
}

func (this *Locker) Range(f func(player *player.Player) bool) {
	for _, p := range this.dict {
		if !f(p) {
			return
		}
	}
}

// Verify 校验待处理操作,空壳跳过,理由同 Select
func (this *Locker) Verify() error {
	for _, p := range this.dict {
		if p.Updater == nil {
			continue
		}
		if err := p.Verify(); err != nil {
			return err
		}
	}
	return nil
}

// Submit 统一提交
//
// ⚠ 空壳跳过:它没有 Updater,也就没有任何待提交的操作。跳过是正确语义、不是掩盖错误——
// 业务侧真的误用了空壳,那一行当场 nil panic,不会走到这里。
func (this *Locker) Submit() error {
	for _, p := range this.dict {
		if p.Updater == nil {
			continue
		}
		if cc, err := p.Updater.Submit(); err != nil {
			return err
		} else {
			p.Pending.Push(cc...)
		}
	}
	return nil
}
