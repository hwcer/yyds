package actor

import (
	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/yyds/players/player"
)

var w *await.Await

type Args struct {
	self   string
	uid    []string
	args   any
	handle player.LockerHandle
}

// NewLocker 已经在玩家通道内调用
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

// NewLockerWithLocker 不在玩家通道内，通过全局通道进入
func NewLockerWithLocker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error) {
	msg := &Args{uid: uid, handle: handle, args: args}
	l := &Locker{done: done}
	return w.Call(l.call, msg)
}

type Locker struct {
	self string
	dict map[string]*player.Player
	done []func()
}

func (this *Locker) call(args any) (reply any, err error) {
	msg, _ := args.(*Args)
	this.self = msg.self
	for _, v := range msg.uid {
		if err = this.loading(v); err != nil {
			return
		}
	}
	defer this.release()
	return msg.handle(this, msg.args)
}

func (this *Locker) release() {
	for uid, p := range this.dict {
		p.Release()
		if uid != this.self {
			p.Unlock()
		}
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
		r = newPlayer(uid)
	}

	if uid != this.self {
		// 非自己：通过目标玩家的 channel 加载，确保数据安全
		err = invoke(r, func() error {
			if e := r.Loading(false); e != nil {
				instance.Manage.Delete(uid)
				return e
			}
			r.Reset()
			return nil
		})
		if err != nil {
			return err
		}
		// 加载完成后 Lock 占住目标 actor，直到 release 时 Unlock
		r.Lock()
	} else {
		// 自己：已在自己的 actor 协程内，直接操作
		if err = r.Loading(false); err != nil {
			instance.Manage.Delete(uid)
			return err
		}
		r.Reset()
	}

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
