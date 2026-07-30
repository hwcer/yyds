package players

import (
	"fmt"
	"sync/atomic"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/yyds/players/actor"
	"github.com/hwcer/yyds/players/emitter"
	"github.com/hwcer/yyds/players/locker"
	"github.com/hwcer/yyds/players/player"
)

var (
	playersOnline    atomic.Int32 //在线人数
	playersMemory    atomic.Int32 //当前缓存总量(含离线未释放),daemon 每 tick 刷新
	playersRecycling map[string]*player.Player
)

var ps Players
var newSyncer func() player.Syncer

func Start() error {
	if !playersState.CompareAndSwap(stateStopped, stateRunning) {
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

// Terminate 强制玩家下线
//
// 迁入拒绝态 StatusTerminated:接入层从此拒绝该玩家一切请求(见 player.Denied),
// Connected() 也不再让他上线,随后由 daemon 在下一个 tick 释放玩家对象。
// 必须靠状态而不是超时 —— context 层在任何判断之前就会 KeepAlive 刷新心跳,
// 只改心跳的话玩家发一个包就把踢人无声撤销了。
//
// **调用方必须持有该玩家的锁**(和 player.Send 同一契约):要在这里补发下线事件,
// 就得碰 p.Updater。业务层操作 c.Player 时天然满足;踢别人套一层
// Get(uid, func(p){ Terminate(p); return nil })。
// 不要改成调 disconnect():它内部还会 p.Lock() 一次,mutex 不可重入,直接自锁死。
//
// 收 *player.Player 而不是 uid:调用方手上基本都有玩家对象,省一次查表,
// 也与同包的 Connected(p, meta) 一致。
//
// 不断开网关连接(网关未提供该接口),客户端收到拒绝码后需自行回登录流程。
// 释放后重新登录是允许的,永久封禁要靠业务侧落库标记(如 role.ban)在登录路径拦。
func Terminate(p *player.Player) bool {
	if p == nil {
		return false
	}
	//覆盖所有还能重新上线的状态(None/Connected/Disconnect/Offline):Connected() 允许
	//从 None/Disconnect/Offline 复活,只踢在线玩家的话掉线但仍在内存里的会顶回来。
	//必须 CAS 而不是 Store:Load 与写入之间 released() 可能已把状态翻成 Released,
	//Store 会覆盖回 Terminated,导致 Updater 销毁后又走一遍释放而空指针
	var from int32
	for {
		from = atomic.LoadInt32(&p.Status)
		if p.Denied(from) {
			//Released 无需再踢;Locked 说明 Loading 正在锁内借用状态并会还原,需稍后重试
			return false
		}
		if atomic.CompareAndSwapInt32(&p.Status, from, player.StatusTerminated) {
			break
		}
	}
	//补发欠着的下线事件,按原状态区分避免重复:
	//Connected 欠 Disconnect+Offline,Disconnect 只欠 Offline,None/Offline 不欠
	if from == player.StatusConnected {
		playersOnline.Add(-1)
		emitter.Events.Emit(p.Updater, EventDisconnect)
	}
	if from == player.StatusConnected || from == player.StatusDisconnect {
		emitter.Events.Emit(p.Updater, EventOffline)
	}
	return true
}

// Get 获取在线玩家, 注意返回NIL时,加锁失败或者玩家未登录,已经对Player加锁
// 不进行初始化，数据按需模式读写
func Get(uid string, handle player.Handle) error {
	if err := available(); err != nil {
		return err
	}
	return ps.Get(uid, handle)
}

// Load 加载玩家数据,如果不在线则实时读写数据库
// init 是否立即初始化所有数据
func Load(uid string, handle player.Handle) (err error) {
	if err := available(); err != nil {
		return err
	}
	return ps.Load(uid, false, handle)
}

// Login 登录成功,只能在登录时调用
// test 为 true 时以测试模式登录（不写库）
func Login(uid string, test bool, meta map[string]string, handle player.Handle) (err error) {
	if err := available(); err != nil {
		return err
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
	if err := available(); err != nil {
		return nil, err
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
