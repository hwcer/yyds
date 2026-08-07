package locker

import (
	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

var w *await.Await

type Args struct {
	uid    []string
	args   any
	handle player.LockerHandle
}

func NewLocker(uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	msg := &Args{uid: uid, args: args, handle: handle}
	l := &Locker{done: done}
	return w.Call(l.call, msg)
}

type Locker struct {
	dict map[string]*player.Player
	done []func()
}

func (this *Locker) release() {
	for _, p := range this.dict {
		p.Release()
		p.Unlock()
	}
	for _, d := range this.done {
		d()
	}
	this.dict = nil
}

func (this *Locker) loading(uid string) error {
	if this.dict == nil {
		this.dict = map[string]*player.Player{}
	}

	if _, ok := this.dict[uid]; ok {
		return nil
	}
	r := newPlayer(uid, false)
	r.Lock()
	if i, loaded := instance.Manage.LoadOrStore(r.Key(), r); loaded {
		i.Lock()
		r.Unlock()
		r = i
		//已在内存的也要判拒绝态:Released 的对象 Updater 已被销毁
		if r.Denied() {
			r.Unlock()
			return errors.ErrNotOnline
		}
	}
	//两条分支都要 Loading:原先只在新建分支调,已在内存的直接 Reset ——
	//遇到 players.Lock 留下的空壳(Updater==nil)就是空指针。Loading 幂等,
	//已加载过的对象零成本。actor 版(actor/channel.go)两条分支本来就都调了。
	if err := r.Loading(false); err != nil {
		r.Unlock()
		instance.Manage.Delete(r.Key())
		return err
	}
	r.Reset()
	this.dict[uid] = r
	return nil
}

func (this *Locker) Select(keys ...any) {
	for _, p := range this.dict {
		p.Select(keys...)
	}
}

func (this *Locker) Data() error {
	for _, p := range this.dict {
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

func (this *Locker) Verify() error {
	for _, p := range this.dict {
		if err := p.Verify(); err != nil {
			return err
		}
	}
	return nil
}

// Submit 统一提交
func (this *Locker) Submit() error {
	for _, p := range this.dict {
		if cc, err := p.Updater.Submit(); err != nil {
			return err
		} else {
			p.Pending.Push(cc...)
		}
	}
	return nil
}

func (this *Locker) call(i any) (reply any, err error) {
	args, _ := i.(*Args)
	defer this.release()
	for _, v := range args.uid {
		if err = this.loading(v); err != nil {
			return nil, err
		}
	}
	return args.handle(this, args.args)
}
