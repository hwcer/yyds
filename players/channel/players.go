package channel

import (
	"fmt"
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
	w = await.New(1024, time.Second*5)
	return instance
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
	player.Manage
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
		err = this.load(uid, init, handle)
	default:
		err = fmt.Errorf("channel Players.call args caller error:%v", caller)
	}
	return
}

// 1
func (this *Players) get(uid string, handle player.Handle) error {
	var p *player.Player
	if i, ok := this.Manage.Load(uid); ok {
		p = i
		if p.Status == player.StatusRelease {
			return errors.ErrLoginWaiting
		}
		p.Reset()
		defer p.Release()
	}
	return handle(p)
}

// 2
func (this *Players) load(uid string, init bool, handle player.Handle) (err error) {
	p := player.New(uid)
	if i, loaded := this.Manage.LoadOrStore(uid, p); loaded {
		p = i
		if p.Status == player.StatusRelease {
			return errors.ErrLoginWaiting
		}
	}
	if err = p.Loading(init); err != nil {
		this.Manage.Delete(uid)
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

func (this *Players) Load(uid string, init bool, handle player.Handle) (err error) {
	args := playerAwaitArgs{}
	args[playerAwaitArgsUid] = uid
	args[playerAwaitArgsInit] = init
	args[playerAwaitArgsCaller] = int8(2)
	args[playerAwaitArgsHandle] = handle
	_, err = w.Call(this.call, args)
	return err
}

func (this *Players) Locker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error) {
	return NewLocker(uid, handle, args, done...)
}
