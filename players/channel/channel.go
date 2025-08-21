package channel

import (
	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/yyds/players/player"
)

//func NewMulti(readOnly bool) *Locker {
//	return &Locker{dict: map[uint64]*players.Player{}, readOnly: readOnly}
//}

var w *await.Await

type Args struct {
	uid    []string
	args   any
	handle player.LockerHandle
}

// 已经在控制携程内
func NewLocker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error) {
	msg := &Args{uid: uid, handle: handle, args: args}
	l := &Locker{done: done}
	return l.call(msg)
}

// NewLockerWithLocker 通常在脚本或者不在控制携程之内才使用这个方法先进入防并发携程
func NewLockerWithLocker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error) {
	msg := &Args{uid: uid, handle: handle, args: args}
	l := &Locker{done: done}
	return w.Call(l.call, msg)
}

type Locker struct {
	dict map[string]*player.Player
	done []func()
}

func (this *Locker) call(args any) (reply any, err error) {
	msg, _ := args.(*Args)
	for _, v := range msg.uid {
		if err = this.loading(v); err != nil {
			return
		}
	}
	defer this.release()
	return msg.handle(this, msg.args)
}

func (this *Locker) release() {
	for _, p := range this.dict {
		p.Release()
	}
	for _, d := range this.done {
		d()
	}
	this.dict = nil
}

func (this *Locker) loading(uid string) (err error) {
	if this.dict == nil {
		this.dict = map[string]*player.Player{}
	}
	if _, ok := this.dict[uid]; ok {
		return nil
	}
	var r *player.Player
	if i, ok := instance.Manage.Load(uid); ok {
		r = i
	} else {
		r = player.New(uid)
	}
	//未初始化
	if err = r.Loading(false); err != nil {
		instance.Manage.Delete(uid)
		return err
	}
	r.Reset()
	this.dict[uid] = r
	return
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
