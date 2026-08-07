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
	if !ok {
		return errors.ErrNotOnline
	}
	return invoke(p, func() error {
		//状态判定必须在通道内、Reset 之前:released 会在通道内把 Updater 置空
		if p.Denied(atomic.LoadInt32(&p.Status)) {
			return errors.ErrNotOnline
		}
		p.Reset()
		defer p.Release()
		return handle(p)
	})
}

// Load 加载玩家并进入通道执行
func (this *Players) Load(uid string, test bool, handle player.Handle) error {
	r := newPlayer(uid, test)
	if i, loaded := this.Manage.LoadOrStore(r.Key(), r); loaded {
		r = i
		if r.Denied(atomic.LoadInt32(&r.Status)) {
			return errors.ErrLoginWaiting
		}
	}
	return invoke(r, func() error {
		if err := r.Loading(test); err != nil {
			this.Manage.Delete(r.Key())
			return err
		}
		r.Reset()
		defer r.Release()
		return handle(r)
	})
}

// Lock 独占该玩家但**不加载任何数据**,详见 players.Lock 的说明。
func (this *Players) Lock(uid string, handle player.Handle) error {
	r := newPlayer(uid, false)
	i, loaded := this.Manage.LoadOrStore(r.Key(), r)
	if loaded {
		r = i
	} else {
		//空壳用完即摘,理由同 locker 版;Delete/Close 必须在通道外做
		//(Close 关的是玩家通道,持有时关会让 worker 卡死),故放 defer
		defer func() {
			this.Manage.Delete(r.Key())
			r.Close()
		}()
	}
	return invoke(r, func() error {
		if loaded && r.Denied(atomic.LoadInt32(&r.Status)) {
			return errors.ErrNotOnline
		}
		//不调 Reset/Release,见 locker 版注释
		return handle(r)
	})
}

func (this *Players) Locker(self string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	return NewLocker(self, uid, args, handle, done...)
}
