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
	//必须先判状态再 Reset:拿到锁时可能已经被 released 销毁。
	//Updater==nil 是 Lock 留下的空壳,按定义不是在线玩家,理由见 actor 版同处注释。
	if p.Denied(atomic.LoadInt32(&p.Status)) || p.Updater == nil {
		return errors.ErrNotOnline
	}
	p.Reset()
	defer p.Release()
	return handle(p)
}

// Load 加载玩家;init=false 时只占锁位不加载数据,详见 players.Load
func (this *Players) Load(uid string, test, init bool, handle player.Handle) (err error) {
	r := newPlayer(uid, test)
	r.Lock()
	i, loaded := this.Manage.LoadOrStore(r.Key(), r)
	if loaded {
		r.Unlock()
		r = i
		r.Lock()
	} else if !init {
		//空壳必须补一次心跳,理由见 actor 版同处注释
		r.KeepAlive(0)
	}
	defer r.Unlock()
	if !init {
		//只占锁位:不 Loading。理由与 Reset/Release 仍照做的原因见 actor 版同处注释。
		if r.Denied() {
			return errors.ErrNotOnline
		}
		r.Reset()
		defer r.Release()
		return handle(r)
	}
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
