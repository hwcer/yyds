package locker

import (
	"github.com/hwcer/yyds/kernel/players/player"
	"github.com/hwcer/yyds/kernel/share"
	"sync"
	"time"
)

var (
	instance = Players{dict: sync.Map{}}
)

func Start() *Players {
	w.Start(10, time.Second*5)
	return &instance
}

type Players struct {
	dict sync.Map
}

func (this *Players) Try(uid uint64, handle player.Handle) error {
	var p *player.Player
	if v, ok := this.dict.Load(uid); ok {
		p = v.(*player.Player)
		if ok = p.TryLock(); ok {
			defer p.Unlock()
			p.Reset()
			defer p.Release()
		} else {
			p = nil
		}
	}
	if p != nil && p.Status == player.StatusRelease {
		return share.ErrLoginWaiting
	}
	return handle(p)
}

func (this *Players) Get(uid uint64, handle player.Handle) error {
	var p *player.Player
	if v, ok := this.dict.Load(uid); ok {
		p = v.(*player.Player)
		p.Lock()
		defer p.Unlock()
		p.Reset()
		defer p.Release()
	}
	if p != nil && p.Status == player.StatusRelease {
		return share.ErrLoginWaiting
	}
	return handle(p)
}

func (this *Players) Load(uid uint64, init bool, handle player.Handle) (err error) {
	r := player.New(uid)
	r.Lock()
	defer r.Unlock()
	if i, loaded := this.dict.LoadOrStore(uid, r); loaded {
		np := i.(*player.Player)
		np.Lock()
		defer np.Unlock()
		r = np
	}
	//未初始化
	if err = r.Loading(init); err != nil {
		this.dict.Delete(uid)
		return
	}
	r.Reset()
	defer r.Release()
	return handle(r)
}

func (this *Players) Range(f func(uint64, *player.Player) bool) {
	this.dict.Range(func(key, value any) bool {
		return f(key.(uint64), value.(*player.Player))
	})
}

// Store 存储玩家对象，用于初始化
func (this *Players) Store(k uint64, v *player.Player) {
	this.dict.Store(k, v)
}
func (this *Players) Delete(k uint64) {
	this.dict.Delete(k)
}

func (this *Players) Locker(uid []uint64, handle player.LockerHandle, done ...func()) error {
	return NewLocker(uid, handle, done...)
}

// LoadWithUnlock 获取无锁状态的Player,无锁,无状态判断,仅仅API入口处使用
func (this *Players) LoadWithUnlock(uid uint64) (r *player.Player) {
	v, ok := this.dict.Load(uid)
	if ok {
		r = v.(*player.Player)
	}
	return
}
