package players

import (
	"sync/atomic"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/yyds/players/emitter"
	"github.com/hwcer/yyds/players/player"
)

var (
	playersOnline    atomic.Int32 //在线人数
	playersMemory    atomic.Int32 //当前缓存总量(含离线未释放),daemon 每 tick 刷新
	playersRecycling map[string]*player.Player
)


func Start() error {
	if !playersState.CompareAndSwap(stateStopped, stateRunning) {
		return nil
	}
	//🔴 ReleaseBatch 的零值是**致命**的,与其它 Options 不同:一个玩家都放不掉,
	//内存只涨不跌,而现象只是"跑久了内存回不去"。MemoryPlayer 这类零值只是策略激进,
	//不会把功能打死,所以只有它需要兜底。
	if Options.ReleaseBatch <= 0 {
		Options.ReleaseBatch = 100
	}
	newManage()
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
	return get(uid, handle)
}

// Load 加载玩家数据,如果不在线则实时读写数据库。
//
// init 传 **false** 表示「只占锁位,不加载数据」:玩家不在内存时只放一个
// 空壳进管理器占住锁位,回调里拿到的 p.Updater 是 **nil**。用于「数据不经 Updater、但仍要
// 与玩家其它操作互斥」的场景——运营后台 / GM 工具直连数据库改档、离线数据修复等。这类操作
// 本来就不读内存数据,按默认走一遍等于白白从库里拉一遍全量、还把离线玩家长期驻留进内存。
//
// 空壳**留在管理器里**,不会用完就删——处理期间别人(登录 / Get / Load)可能已经拿到同一个
// 指针正在等锁,删掉等于让它在管理器之外操作孤儿对象,打拒绝态则会把一次合法登录拒了。
// 空壳的归宿是**自愈**:谁需要数据谁调 Loading 把它补全(本方法 init=true 时天然如此),
// 没人来就由 daemon 正常回收。Get 把空壳认作"不在线"返回 ErrNotOnline,于是既有的
// 「Get 失败 → 退回 Load」降级链会自动完成这次自愈。
//
// init=false 与 init=true 的唯一差别是**不预加载数据**,不是"对象处于半初始化状态":
// Reset/Release 照常执行(对空壳是空操作)。所以回调里:
//   - p.Updater == nil 表示玩家不在内存 —— 用它就是当场 nil panic,不会静默出错
//   - p.Updater != nil 表示玩家本就在内存,且**已为本次调用 Reset 过**,可以正常使用
//   - 中途发现需要数据 → 调 **p.Initialize()**(Loading + Reset 一次做完,幂等),
//     别只调 Loading:Updater 由 New 创建时 now 是零值,漏了 Reset 会得到一个
//     "能用但时间基准是 1 年"的对象,不报错也不 panic
//
// **不要**用 Connected 去"补数据":它是登录入口,会把玩家标成在线(在线数 +1、发
// EventConnect),GM / 后台场景下那是一次假上线。它内部确实也会先 Loading,但那是为了
// 保证"凡是 Connected 的玩家一定有数据"这条不变式,不是给别人当自愈入口用的。
func Load(uid string, init bool, handle player.Handle) (err error) {
	if err = available(); err != nil {
		return err
	}
	return load(uid, false, init, handle)
}

// Login 登录成功,只能在登录时调用
// test 为 true 时以测试模式登录（不写库）
func Login(uid string, test bool, meta map[string]string, handle player.Handle) (err error) {
	if err = available(); err != nil {
		return err
	}
	err = load(uid, test, true, func(p *player.Player) error {
		if e := Connected(p, meta); e != nil {
			return e
		}
		return handle(p)
	})
	return
}

// Locker 批量取得多个玩家的操作权限
//
// 有 Context 的地方(业务 handler 内)请用 **c.Mutex().Lock / Async** —— 它们在这之上多做了
// 一件必须做的事:先把自己这一侧 Submit 结清并让出锁。本函数留给 daemon、定时器、
// 内网 RPC 这类拿不到 Context、也不持有任何玩家锁的调用方。
//
// 🔴 **调用方绝不能持有任何玩家锁**:批量锁进去之后会逐个抢目标玩家的锁,
// 攥着自己的去抢别人的就是标准 ABBA。
//
// ⚠ 取锁阶段整批成败:uids 里任何一个取不到(非法 uid / 拒绝态 / 排队超时),
// 整个回调都不会执行 —— 包括那些本来没问题的玩家。
//
// 回调里拿到的是 player.Locker:**不保证玩家在线、也不保证有数据**,不在内存的给的是
// 空壳(p.Updater == nil),完整契约见 player.Locker 接口上的说明。
func Locker(uid []string, args any, handle player.LockerHandle, done ...func()) (any, error) {
	if err := available(); err != nil {
		return nil, err
	}
	return newBatch(uid, args, handle, done...)
}

func Range(f func(string, *player.Player) bool) {
	manage.Range(f)
}

// NewPlayer 造一个带并发控制器的玩家对象(尚未进入管理器)
//
// 只有预加载(preload.go)这类"对象还没被别人拿到、可以安全地先 Loading 再 Store"的路径才用它。
// 正常取玩家一律走 Get / Load。
func NewPlayer(uid string, test bool) *player.Player {
	return player.New(uid, test)
}
