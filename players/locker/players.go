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
	p, ok := this.Manage.Load(uid)
	if !ok {
		return errors.ErrNotOnline
	}
	p.Lock()
	defer p.Unlock()
	//必须先判状态再 Reset:拿到锁时可能已经被 released 销毁
	if p.Denied(atomic.LoadInt32(&p.Status)) {
		return errors.ErrNotOnline
	}
	p.Reset()
	defer p.Release()
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

// Lock 独占该玩家但**不加载任何数据**,详见 players.Lock 的说明。
func (this *Players) Lock(uid string, handle player.Handle) error {
	r := newPlayer(uid, false)
	r.Lock()
	i, loaded := this.Manage.LoadOrStore(r.Key(), r)
	if loaded {
		r.Unlock()
		r = i
		r.Lock()
	}
	defer r.Unlock()
	if loaded {
		//已在内存:与 Get 同一口径,拒绝态不再交出去
		if r.Denied(atomic.LoadInt32(&r.Status)) {
			return errors.ErrNotOnline
		}
	} else {
		//我们建的空壳 Updater==nil,绝不能留在 Manage 里:
		//后续 Get/Load 会对它 Reset(),那是 p.Updater.Reset(),直接空指针
		defer this.Manage.Delete(r.Key())
	}
	//不调 Reset/Release:两者都直接解引用 Updater,空壳上必崩;
	//已在内存的那条也不 Reset,本方法只保证互斥,不提供数据访问
	return handle(r)
}

func (this *Players) Locker(_ string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	return NewLocker(uid, args, handle, done...)
}
