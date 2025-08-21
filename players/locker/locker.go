package locker

import (
	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/yyds/players/player"
)

var w *await.Await

type Args struct {
	uid    []string
	args   any
	handle player.LockerHandle
}

func NewLocker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error) {
	msg := &Args{uid: uid, handle: handle, args: args}
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
	r := player.New(uid)
	r.Lock()
	if i, loaded := instance.Manage.LoadOrStore(uid, r); loaded {
		r.Unlock()
		r = i
		r.Lock()
	}
	//未初始化
	if err := r.Loading(false); err != nil {
		r.Unlock()
		instance.Manage.Delete(uid)
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
		if err := p.Updater.Verify(); err != nil {
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
			p.Dirty.Push(cc...)
		}
	}
	return nil
}

func (this *Locker) call(i any) (reply any, err error) {
	args, _ := i.(*Args)
	for _, v := range args.uid {
		if err = this.loading(v); err != nil {
			return nil, err
		}
	}
	defer this.release()
	return args.handle(this, args.args)
}
