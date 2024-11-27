package players

import (
	"fmt"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/logger"
	"github.com/hwcer/scc"
	"net"
	"server/define"
	"server/game/model"
	"server/game/players/channel"
	"server/game/players/locker"
	"server/game/players/options"
	"server/game/players/player"
	"sync/atomic"
)

var (
	playersOnline      int32 //在线人数
	playersStarted     int32
	playersRecycling   map[uint64]*player.Player
	playersReleaseTime int //距离上次内存清理的事件间隔
)

var ps Players

func Start() error {
	if !atomic.CompareAndSwapInt32(&playersStarted, 0, 1) {
		return nil
	}

	if options.Options.AsyncModel == options.AsyncModelLocker {
		ps = locker.Start()
	} else if options.Options.AsyncModel == options.AsyncModelChannel {
		ps = channel.Start()
	} else {
		return fmt.Errorf("players: invalid options")
	}
	scc.CGO(daemon)
	if err := loading(); err != nil {
		return err
	}
	return nil
}
func Online() int32 {
	return playersOnline
}

// Try 获取在线玩家, 使用TryLock 尝试获得锁
func Try(uid uint64, handle player.Handle) error {
	return ps.Try(uid, handle)
}

// Get 获取在线玩家, 注意返回NIL时,加锁失败或者玩家未登录,已经对Player加锁
func Get(uid uint64, handle player.Handle) error {
	return ps.Get(uid, handle)
}

// Load 加载玩家数据,如果不在线则实时读写数据库
// init 是否立即初始化所有数据
func Load(uid uint64, init bool, handle player.Handle) (err error) {
	return ps.Load(uid, init, handle)
}

// Login 登录成功,只能在登录时调用
//
//	TODO 顶号
func Login(uid uint64, conn net.Conn, handle player.Handle) (err error) {
	err = ps.Load(uid, true, func(p *player.Player) error {
		if !Connected(p, conn) {
			return define.ErrLoginWaiting
		}
		return handle(p)
	})
	return
}

func Locker(uid []uint64, handle player.LockerHandle, done ...func()) error {
	return ps.Locker(uid, handle, done...)
}

// LoadWithUnlock 获取无锁状态的Player,无锁,无状态判断,仅仅API入口处使用
//func LoadWithUnlock(uid uint64) (r *player.Player) {
//	return ps.LoadWithUnlock(uid)
//}

// loading 初始加载用户到内存
func loading() (err error) {
	if options.Options.MemoryInstall == 0 {
		return nil
	}
	var rows []*model.Role
	now := times.Unix()
	lastTime := now - 7*86400
	tx := model.DB.Select("_id", "name").Order("update", -1)
	tx = tx.Where("update > ?", lastTime)
	tx = tx.Limit(options.Options.MemoryInstall).Find(&rows)
	if tx.Error != nil {
		return tx.Error
	}
	var p *player.Player

	for _, r := range rows {
		p = player.New(r.Uid)
		if err = p.Loading(true); err == nil {
			ps.Store(r.Uid, p)
			p.KeepAlive(now)
			role := p.Role.All()
			logger.Debug("预加载用户: [%v] %v", role.Uid, role.Guid)
		}
	}
	logger.Trace("累计预加载用户数量:%v\n", len(rows))
	return
}
