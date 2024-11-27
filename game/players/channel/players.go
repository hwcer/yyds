package channel

import (
	"fmt"
	"server/define"
	"server/game/players/player"
	"sync"
	"time"
)

var (
	instance = Players{dict: sync.Map{}}
)

func Start() *Players {
	w.Start(1024, time.Second*5)
	return &instance
}

type playerAwaitArgs map[playerAwaitArgsKey]any
type playerAwaitArgsKey int8

const (
	playerAwaitArgsUid playerAwaitArgsKey = iota
	playerAwaitArgsInit
	playerAwaitArgsHandle
	playerAwaitArgsOnline //只获取在线玩家
)

type Players struct {
	dict sync.Map
}

func (this *Players) call(args any) (reply any, err error) {
	msg, _ := args.(playerAwaitArgs)
	if msg == nil {
		return nil, fmt.Errorf("channel Players.call args error:%v", args)
	}
	uid := msg[playerAwaitArgsUid].(uint64)
	init := msg[playerAwaitArgsInit].(bool)
	handle := msg[playerAwaitArgsHandle].(func(player2 *player.Player) error)
	online := msg[playerAwaitArgsOnline].(bool)
	if online {
		err = this.getOnline(uid, handle)
	} else {
		err = this.getPlayer(uid, init, handle)
	}
	return
}

func (this *Players) get(uid uint64, init bool) (p *player.Player, err error) {
	p = player.New(uid)
	if i, loaded := this.dict.LoadOrStore(uid, p); loaded {
		p = i.(*player.Player)
	} else if err = p.Loading(init); err != nil {
		this.dict.Delete(uid)
		return nil, err
	}
	return
}

func (this *Players) getPlayer(uid uint64, init bool, handle player.Handle) error {
	p, err := this.get(uid, init)
	if err != nil {
		return err
	}
	if p.Status == player.StatusRelease {
		return define.ErrLoginWaiting
	}
	p.Reset()
	defer p.Release()
	return handle(p)
}
func (this *Players) getOnline(uid uint64, handle player.Handle) error {
	var p *player.Player
	if i, ok := this.dict.Load(uid); ok {
		p = i.(*player.Player)
		if p.Status == player.StatusRelease {
			return define.ErrLoginWaiting
		}
		p.Reset()
		defer p.Release()
	}
	return handle(p)
}
func (this *Players) Try(uid uint64, handle player.Handle) (err error) {
	return this.Get(uid, handle)
}
func (this *Players) Get(uid uint64, handle player.Handle) (err error) {
	args := playerAwaitArgs{}
	args[playerAwaitArgsUid] = uid
	args[playerAwaitArgsInit] = false
	args[playerAwaitArgsHandle] = handle
	args[playerAwaitArgsOnline] = false
	_, err = w.Call(this.call, args)
	return err
}

func (this *Players) Load(uid uint64, init bool, handle player.Handle) (err error) {
	args := playerAwaitArgs{}
	args[playerAwaitArgsUid] = uid
	args[playerAwaitArgsInit] = init
	args[playerAwaitArgsOnline] = false
	args[playerAwaitArgsHandle] = handle
	_, err = w.Call(this.call, args)
	return err
}

//func (this *Players) Login(uid uint64, conn net.Conn, handle player.Handle) (err error) {
//	args := playerAwaitArgs{}
//	args[playerAwaitArgsUid] = uid
//	args[playerAwaitArgsInit] = true
//	args[playerAwaitArgsOnline] = false
//	args[playerAwaitArgsHandle] = func(p *player.Player) error {
//		if !p.Connected(conn) {
//			return define.ErrLoginWaiting
//		}
//		return handle(p)
//	}
//	_, err = w.Call(this.call, args)
//	return
//}

func (this *Players) Range(f func(uint64, *player.Player) bool) {
	this.dict.Range(func(key, value interface{}) bool {
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
//func (this *Players) LoadWithUnlock(uid uint64) (r *player.Player) {
//	v, ok := this.dict.Load(uid)
//	if ok {
//		r = v.(*player.Player)
//	}
//	return
//}
