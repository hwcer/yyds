package players

import (
	"fmt"
	"sync/atomic"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/channel"
	"github.com/hwcer/yyds/players/locker"
	"github.com/hwcer/yyds/players/player"
)

var (
	playersOnline    int32 //在线人数
	playersMemory    int32 //当前缓存总量
	playersStarted   int32
	playersRecycling map[string]*player.Player
	//playersReleaseTime int //距离上次内存清理的事件间隔
)

var ps Players

func Start() error {
	if !atomic.CompareAndSwapInt32(&playersStarted, 0, 1) {
		return nil
	}
	//cosgo.On(cosgo.EventTypStarted, loading)
	if Options.AsyncModel == AsyncModelLocker {
		ps = locker.New()
	} else if Options.AsyncModel == AsyncModelChannel {
		ps = channel.New()
	} else {
		return fmt.Errorf("players: invalid options")
	}
	scc.CGO(daemon)
	return loading()
}
func Online() int32 {
	return playersOnline
}

// Try 获取在线玩家, 使用TryLock 尝试获得锁
//func Try(uid string, handle player.Handle) error {
//	return ps.Try(uid, handle)
//}

// Get 获取在线玩家, 注意返回NIL时,加锁失败或者玩家未登录,已经对Player加锁
// 不进行初始化，数据按需模式读写
func Get(uid string, handle player.Handle) error {
	if playersStarted == 0 {
		return errors.ErrServerClosed
	}
	return ps.Get(uid, handle)
}

// Load 加载玩家数据,如果不在线则实时读写数据库
// init 是否立即初始化所有数据
func Load(uid string, init bool, handle player.Handle) (err error) {
	if playersStarted == 0 {
		return errors.ErrServerClosed
	}
	return ps.Load(uid, init, handle)
}

// Login 登录成功,只能在登录时调用
func Login(uid string, meta map[string]string, handle player.Handle) (err error) {
	if playersStarted == 0 {
		return errors.ErrServerClosed
	}
	err = ps.Load(uid, true, func(p *player.Player) error {
		if e := Connected(p, meta); e != nil {
			return e
		}
		return handle(p)
	})
	return
}

func Locker(uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	if playersStarted == 0 {
		return nil, errors.ErrServerClosed
	}
	return ps.Locker(uid, args, handle, done...)
}

func Range(f func(string, *player.Player) bool) {
	ps.Range(f)
}

//// Disconnect 下线,心跳超时,断开连接等
//func Disconnect(p *player.Player) bool {
//	status := p.Status
//	if status != player.StatusConnected {
//		return false
//	}
//	if !atomic.CompareAndSwapInt32(&p.Status, player.StatusConnected, player.StatusDisconnect) {
//		return false
//	}
//	p.KeepAlive(0)
//	atomic.AddInt32(&playersOnline, -1)
//	updater.Emit(p.Updater, player.EventDisconnect)
//	return true
//}
//
//// Offline 业务逻辑层面掉线
//func Offline(p *player.Player) bool {
//	status := p.Status
//	if !(status == player.StatusNone || status == player.StatusDisconnect) {
//		return false
//	}
//	if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusOffline) {
//		return false
//	}
//	p.KeepAlive(0)
//	return true
//}
