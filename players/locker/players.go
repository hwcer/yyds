package locker

import (
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
	if p != nil && p.Status == player.StatusRelease {
		return errors.ErrLoginWaiting
	}
	return handle(p)
}

func (this *Players) Load(uid string, init bool, handle player.Handle) (err error) {
	r := player.New(uid)
	r.Lock()
	defer r.Unlock()
	if i, loaded := this.Manage.LoadOrStore(uid, r); loaded {
		r = i
		r.Lock()
		defer r.Unlock()
	}
	if err = r.Loading(init); err != nil {
		this.Manage.Delete(uid)
		return
	}
	r.Reset()
	defer r.Release()
	return handle(r)
}

func (this *Players) Locker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error) {
	return NewLocker(uid, handle, args, done...)
}
