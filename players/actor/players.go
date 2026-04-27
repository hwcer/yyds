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

func New() *Players {
	w = await.New(10, time.Second*5)
	return instance
}

type Players struct {
	player.Manage
}

func invoke(p *player.Player, fn func() error) error {
	return p.Syncer.(*Syncer).invoke(fn)
}

// Get 只获取在线玩家，进入玩家通道执行
func (this *Players) Get(uid string, handle player.Handle) error {
	p, ok := this.Manage.Load(uid)
	if !ok || atomic.LoadInt32(&p.Status) == player.StatusReleased {
		return errors.ErrNotOnline
	}
	return invoke(p, func() error {
		p.Reset()
		defer p.Release()
		return handle(p)
	})
}

// Load 加载玩家并进入通道执行
func (this *Players) Load(uid string, init bool, handle player.Handle) error {
	r := newPlayer(uid)
	if i, loaded := this.Manage.LoadOrStore(uid, r); loaded {
		r = i
		if atomic.LoadInt32(&r.Status) == player.StatusReleased {
			return errors.ErrLoginWaiting
		}
	}
	return invoke(r, func() error {
		if err := r.Loading(init); err != nil {
			this.Manage.Delete(uid)
			return err
		}
		r.Reset()
		defer r.Release()
		return handle(r)
	})
}

func (this *Players) Locker(self string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	return NewLocker(self, uid, args, handle, done...)
}
