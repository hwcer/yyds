package locker

import (
	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/yyds/players/player"
)

var w *await.Await

type Args struct {
	uid    []uint64
	done   []func()
	handle player.LockerHandle
}

func NewLocker(uid []uint64, handle player.LockerHandle, done ...func()) (any, error) {
	msg := &Args{uid: uid, handle: handle, done: done}
	l := &Locker{}
	return w.Call(l.call, msg)
}

type Locker struct {
	dict map[uint64]*player.Player
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

func (this *Locker) loading(uid uint64) error {
	if this.dict == nil {
		this.dict = map[uint64]*player.Player{}
	}
	if _, ok := this.dict[uid]; ok {
		return nil
	}
	r := player.New(uid)
	r.Lock()
	if i, loaded := instance.dict.LoadOrStore(uid, r); loaded {
		r.Unlock()
		r = i.(*player.Player)
		r.Lock()
	}
	//未初始化
	if err := r.Loading(false); err != nil {
		r.Unlock()
		instance.dict.Delete(uid)
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

func (this *Locker) Get(uid uint64) *player.Player {
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

func (this *Locker) call(args any) (reply any, err error) {
	msg, _ := args.(*Args)
	bw := &Locker{done: msg.done}
	for _, v := range msg.uid {
		if err = bw.loading(v); err != nil {
			return nil, err
		}
	}
	defer bw.release()
	return msg.handle(bw)
}
