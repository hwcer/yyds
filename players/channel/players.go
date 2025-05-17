package channel

import (
	"fmt"
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
	w = await.New(1024, time.Second*5)
	return &instance
}

type playerAwaitArgs map[playerAwaitArgsKey]any
type playerAwaitArgsKey int8

const (
	playerAwaitArgsUid playerAwaitArgsKey = iota
	playerAwaitArgsInit
	playerAwaitArgsCaller //内部方法
	playerAwaitArgsHandle //回调业务逻辑
)

type Players struct {
	dict sync.Map
}

func (this *Players) call(args any) (reply any, err error) {
	msg, _ := args.(playerAwaitArgs)
	if msg == nil {
		return nil, fmt.Errorf("channel Players.call args error:%v", args)
	}
	uid := msg[playerAwaitArgsUid].(string)
	init := msg[playerAwaitArgsInit].(bool)
	caller := msg[playerAwaitArgsCaller].(int8)
	handle := msg[playerAwaitArgsHandle].(player.Handle)
	switch caller {
	case 1:
		err = this.get(uid, handle)
	case 2:
		err = this.try(uid, handle)
	case 3:
		err = this.load(uid, init, handle)
	default:
		err = fmt.Errorf("channel Players.call args caller error:%v", caller)
	}
	return
}

// 1
func (this *Players) get(uid string, handle player.Handle) error {
	var p *player.Player
	if i, ok := this.dict.Load(uid); ok {
		p = i.(*player.Player)
		if p.Status == player.StatusRelease {
			return errors.ErrLoginWaiting
		}
		p.Reset()
		defer p.Release()
	}
	return handle(p)
}

// 2
func (this *Players) try(uid string, handle player.Handle) (err error) {
	p := player.New(uid)
	if i, loaded := this.dict.LoadOrStore(uid, p); loaded {
		p = i.(*player.Player)
		if p.Status == player.StatusRelease {
			return errors.ErrLoginWaiting
		}
	} else if err = p.Loading(true); err != nil {
		this.dict.Delete(uid)
		return err
	}

	p.Reset()
	defer p.Release()
	return handle(p)
}

// 3
func (this *Players) load(uid string, init bool, handle player.Handle) (err error) {
	p := player.New(uid)
	if i, loaded := this.dict.LoadOrStore(uid, p); loaded {
		p = i.(*player.Player)
		if p.Status == player.StatusRelease {
			return errors.ErrLoginWaiting
		}
	}
	if err = p.Loading(init); err != nil {
		this.dict.Delete(uid)
		return err
	}
	p.Reset()
	defer p.Release()
	return handle(p)
}
func (this *Players) Get(uid string, handle player.Handle) (err error) {
	args := playerAwaitArgs{}
	args[playerAwaitArgsUid] = uid
	args[playerAwaitArgsInit] = false
	args[playerAwaitArgsCaller] = int8(1)
	args[playerAwaitArgsHandle] = handle
	_, err = w.Call(this.call, args)
	return err
}
func (this *Players) Try(uid string, handle player.Handle) (err error) {
	args := playerAwaitArgs{}
	args[playerAwaitArgsUid] = uid
	args[playerAwaitArgsInit] = true
	args[playerAwaitArgsCaller] = int8(2)
	args[playerAwaitArgsHandle] = handle
	_, err = w.Call(this.call, args)
	return err
}

func (this *Players) Load(uid string, init bool, handle player.Handle) (err error) {
	args := playerAwaitArgs{}
	args[playerAwaitArgsUid] = uid
	args[playerAwaitArgsInit] = init
	args[playerAwaitArgsCaller] = int8(3)
	args[playerAwaitArgsHandle] = handle
	_, err = w.Call(this.call, args)
	return err
}

func (this *Players) Range(f func(string, *player.Player) bool) {
	this.dict.Range(func(key, value interface{}) bool {
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

func (this *Players) Locker(uid []string, handle player.LockerHandle, done ...func()) (any, error) {
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
