package channel

import (
	"github.com/hwcer/cosgo/await"
	"server/game/players/player"
)

//func NewMulti(readOnly bool) *Locker {
//	return &Locker{dict: map[uint64]*players.Player{}, readOnly: readOnly}
//}

var w = await.New()

type Args struct {
	uid    []uint64
	done   []func()
	handle player.LockerHandle
}

func NewLocker(uid []uint64, handle player.LockerHandle, done ...func()) error {
	msg := &Args{uid: uid, handle: handle, done: done}
	l := &Locker{}
	_, err := w.Call(l.call, msg)
	return err
}

type Locker struct {
	dict map[uint64]*player.Player
	done []func()
}

func (this *Locker) call(args any) (reply any, err error) {
	msg, _ := args.(*Args)
	bw := &Locker{done: msg.done}
	for _, v := range msg.uid {
		if err = bw.loading(v); err != nil {
			bw.release()
			return
		}
	}
	defer bw.release()
	msg.handle(bw)
	return
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

func (this *Locker) loading(uid uint64) (err error) {
	if this.dict == nil {
		this.dict = map[uint64]*player.Player{}
	}
	this.dict[uid], err = instance.get(uid, false)
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
// todo cache ...
func (this *Locker) Submit() error {
	for _, p := range this.dict {
		if _, err := p.Updater.Submit(); err != nil {
			return err
		}
	}
	return nil
}
