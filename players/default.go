package players

import (
	"fmt"
	"sync/atomic"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/actor"
	"github.com/hwcer/yyds/players/locker"
	"github.com/hwcer/yyds/players/player"
)

var (
	playersOnline    atomic.Int32 //在线人数
	playersMemory    atomic.Int32 //当前缓存总量(含离线未释放),daemon 每 tick 刷新
	playersStarted   int32
	playersRecycling map[string]*player.Player
)

var ps Players
var newSyncer func() player.Syncer

func Start() error {
	if !atomic.CompareAndSwapInt32(&playersStarted, 0, 1) {
		return nil
	}
	if Options.AsyncModel == AsyncModelLocker {
		ps = locker.New()
		newSyncer = locker.NewSyncer
	} else if Options.AsyncModel == AsyncModelActor {
		ps = actor.New()
		newSyncer = actor.NewSyncer
	} else {
		return fmt.Errorf("players: invalid options")
	}
	scc.CGO(daemon)
	return loading()
}

// Online 在线人数(StatusConnected)
func Online() int32 {
	return playersOnline.Load()
}

// Memory 内存中的玩家对象总数,含掉线未释放的缓存
// 与 Options.MemoryPlayer / MemoryRelease 对照可判断回收站是否已开始清理
func Memory() int32 {
	return playersMemory.Load()
}

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
func Load(uid string, handle player.Handle) (err error) {
	if playersStarted == 0 {
		return errors.ErrServerClosed
	}
	return ps.Load(uid, false, handle)
}

// Login 登录成功,只能在登录时调用
// test 为 true 时以测试模式登录（不写库）
func Login(uid string, test bool, meta map[string]string, handle player.Handle) (err error) {
	if playersStarted == 0 {
		return errors.ErrServerClosed
	}
	err = ps.Load(uid, test, func(p *player.Player) error {
		if e := Connected(p, meta); e != nil {
			return e
		}
		return handle(p)
	})
	return
}

func Locker(self string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	if playersStarted == 0 {
		return nil, errors.ErrServerClosed
	}
	return ps.Locker(self, uid, args, handle, done...)
}

func Range(f func(string, *player.Player) bool) {
	ps.Range(f)
}

func NewPlayer(uid string, test bool) *player.Player {
	p := player.New(uid, test)
	p.Syncer = newSyncer()
	return p
}
