package players

import (
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

// ============================================================================
// 批量锁:同时取得多个玩家的操作权限
// ============================================================================
//
// 全部动作跑在 w 的**单个** worker 上 —— 全服批量锁排成一队,取锁顺序不可能成环(见 newManage)。
// 公开入口是 Locker(拿不到 Context 的调用方)与 context.Mutex().Lock / Async(业务侧)。

type batchArgs struct {
	uid    []string
	args   any
	handle player.LockerHandle
}

// newBatch 取得这批玩家的操作权限并执行 handle;done 在锁全部归还之后按序执行
func newBatch(uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	msg := &batchArgs{uid: uid, args: args, handle: handle}
	l := &batch{done: done}
	return w.Call(l.call, msg)
}

// batch 实现 player.Locker,完整契约见那个接口上的说明
type batch struct {
	dict map[string]*player.Player
	done []func()
}

func (this *batch) release() {
	for _, p := range this.dict {
		p.Release()
		p.Unlock()
	}
	for _, d := range this.done {
		d()
	}
	this.dict = nil
}

// loading 取得 uid 的操作权限。
//
// 🔴 **只占锁位,不加载数据**,与 load(init=false) 同款。
//
// 批量锁的职责是"取得操作权限",不是"把对方的存档拉起来"。Loading 会把该玩家
// **全部常驻模型整表拉一遍**(RAMTypeMaybe 与 RAMTypeAlways 在 statement.loading()
// 的判据里是等价的,都走 keys=nil 的全量 Getter,"Maybe=按需"是误读),对不在内存的
// 玩家就是"为了改一个字段读了六张表,还把这份存档长期留在内存"——而批量锁的典型用途
// (改对方一两个字段、给对方发条消息)一个字节都用不上。
//
// 于是:玩家在内存就用现成的,不在内存就放一个空壳(Updater==nil)占住锁位。
// 需要数据的调用方**按需自己加载**,契约见 player.Locker 接口上的说明。
func (this *batch) loading(uid string) error {
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
	r := player.New(uid, false)
	r.Lock()
	if i, loaded := manage.LoadOrStore(r.Key(), r); loaded {
		i.Lock()
		r.Unlock()
		r = i
		//已在内存的也要判拒绝态:Released 的对象 Updater 已被销毁
		if r.Denied() {
			r.Unlock()
			return errors.ErrNotOnline
		}
	} else {
		//空壳必须补一次心跳:player.New 不设 heartbeat(恒 0),而 daemon 按
		//Heartbeat()<offlineTime 判回收、内存压力下又按 heartbeat 升序决定先放谁——
		//不刷的话它是全场最先被释放的对象,等不到有人来自愈。与 Players.Load 同款。
		r.KeepAlive(0)
	}
	//Reset 照做(对空壳是空操作):玩家本就在内存时少了这一步,会给出一个"非 nil 但 now
	//是零值"的 Updater——能用、不报错、时间全错。
	//
	//空壳**留在 Manage 里不删**:处理期间别人(登录 / Get / Load)可能已经拿到同一个指针
	//正在等锁,删掉等于让它在 Manage 之外操作孤儿对象。它的归宿是自愈——谁需要数据谁
	//Loading(Players.Load(init=true) / Login / Connected 天然如此),没人来就由 daemon 回收。
	r.Reset()
	this.dict[uid] = r
	return nil
}

// Select 标记待拉取的 key。
//
// ⚠ 空壳(Updater==nil,玩家不在内存)被跳过:它连 Updater 都没有,谈不上"标记待拉取"。
// 要对这类玩家走 Updater,先 p.Initialize() —— 但那会拉起他的全部常驻数据,
// 掂量清楚再用,见 player.Locker 的契约。
func (this *batch) Select(keys ...any) {
	for _, p := range this.dict {
		if p.Updater == nil {
			continue
		}
		p.Select(keys...)
	}
}

// Data 拉取 Select 标记的数据,空壳跳过,理由同 Select
func (this *batch) Data() error {
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

func (this *batch) Get(uid string) *player.Player {
	return this.dict[uid]
}

func (this *batch) Range(f func(player *player.Player) bool) {
	for _, p := range this.dict {
		if !f(p) {
			return
		}
	}
}

// Verify 校验待处理操作,空壳跳过,理由同 Select
func (this *batch) Verify() error {
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
// 业务侧真的误用了空壳(如 cache.GetRole(p.Updater)),那一行当场 nil panic,不会走到这里。
func (this *batch) Submit() error {
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

func (this *batch) call(i any) (reply any, err error) {
	args, _ := i.(*batchArgs)
	defer this.release()
	for _, v := range args.uid {
		if err = this.loading(v); err != nil {
			return nil, err
		}
	}
	return args.handle(this, args.args)
}
