package players

import (
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

// loading 取得 uid 的操作权限,并保证回调拿到的玩家**都有 Updater**。
//
// 玩家在内存就用现成的;不在内存的以 updater **懒加载**建 Updater(Loading(lazy=true)):不预拉,
// 不付整档全量加载(RAMTypeMaybe 与 RAMTypeAlways 在 statement.loading() 里都走全量 Getter,
// "Maybe=按需"是误读)+ 长期驻留的代价——批量锁的典型用途(改对方一两个字段)用不上整档。
// 业务按「对方不在线」写:读前 Select + Data,契约见 player.Locker 接口上的说明。
func (this *batch) loading(uid string) error {
	if this.dict == nil {
		this.dict = map[string]*player.Player{}
	}

	if _, ok := this.dict[uid]; ok {
		return nil
	}
	//取锁位与懒加载和 Players.Load(lazy=true) 共用一份(acquire / prepare,见 manage.go)
	r, err := acquire(uid, false)
	if err != nil {
		return err
	}
	if err = prepare(r, true); err != nil {
		r.Unlock()
		return err
	}
	//Reset 必做:少了这一步会给出一个"非 nil 但 now 是零值"的 Updater——能用、不报错、时间全错
	r.Reset()
	this.dict[uid] = r
	return nil
}

// Select 对全部玩家标记待拉取的 key(懒加载玩家读之前必须 Select + Data,见 player.Locker)
func (this *batch) Select(keys ...any) {
	for _, p := range this.dict {
		p.Select(keys...)
	}
}

// Data 对全部玩家拉取 Select 标记的数据
func (this *batch) Data() error {
	for _, p := range this.dict {
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

// Verify 校验待处理操作
func (this *batch) Verify() error {
	for _, p := range this.dict {
		if err := p.Verify(); err != nil {
			return err
		}
	}
	return nil
}

// Submit 统一提交
func (this *batch) Submit() error {
	for _, p := range this.dict {
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
