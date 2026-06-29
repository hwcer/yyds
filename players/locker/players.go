package locker

import (
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

var (
	instance = &Players{}
)

func init() {
	instance.Manage = *player.NewManage()
}
func New() *Players {
	w = await.New(10, time.Second*5)
	return instance
}

//func (this *Players) Syncer() player.Syncer {
//	return NewSyncer()
//}

type Players struct {
	player.Manage
}

// Get 只获取在线玩家
func (this *Players) Get(uid string, handle player.Handle) error {
	var p *player.Player
	if v, ok := this.Manage.Load(uid); ok {
		p = v
		p.Lock()
		defer p.Unlock()
		p.Reset()
		defer p.Release()
	}
	if p == nil || atomic.LoadInt32(&p.Status) == player.StatusReleased {
		return errors.ErrNotOnline
	}
	return handle(p)
}

func (this *Players) Load(uid string, test bool, handle player.Handle) (err error) {
	r := newPlayer(uid, test)
	r.Lock()
	if i, loaded := this.Manage.LoadOrStore(r.Key(), r); loaded {
		r.Unlock()
		r = i
		r.Lock()
	}
	defer r.Unlock()
	if err = r.Loading(test); err != nil {
		this.Manage.Delete(r.Key())
		return
	}
	r.Reset()
	defer r.Release()
	return handle(r)
}

func (this *Players) Locker(_ string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	return NewLocker(uid, args, handle, done...)
}
