package locker

import (
	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
	"sync"
	"time"
)

var (
	instance = Players{dict: sync.Map{}}
)

func Start() *Players {
	w = await.New(10, time.Second*5)
	return &instance
}

type Players struct {
	dict sync.Map
}

// Get 只获取在线玩家
func (this *Players) Get(uid string, handle player.Handle) error {
	var p *player.Player
	if v, ok := this.dict.Load(uid); ok {
		p = v.(*player.Player)
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
	if i, loaded := this.dict.LoadOrStore(uid, r); loaded {
		r = i.(*player.Player)
		r.Lock()
		defer r.Unlock()
	}
	if err = r.Loading(init); err != nil {
		this.dict.Delete(uid)
		return
	}
	r.Reset()
	defer r.Release()
	return handle(r)
}

func (this *Players) Range(f func(string, *player.Player) bool) {
	this.dict.Range(func(key, value any) bool {
		return f(key.(string), value.(*player.Player))
	})
}

// Store 存储玩家对象，用于初始化
func (this *Players) Store(k string, v *player.Player) {
	this.dict.Store(k, v)
}
func (this *Players) Delete(k string) {
	this.dict.Delete(k)
}

func (this *Players) Locker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error) {
	return NewLocker(uid, handle, args, done...)
}

// LoadWithUnlock 获取无锁状态的Player,无锁,无状态判断,仅仅API入口处使用
func (this *Players) LoadWithUnlock(uid string) (r *player.Player) {
	v, ok := this.dict.Load(uid)
	if ok {
		r = v.(*player.Player)
	}
	return
}
