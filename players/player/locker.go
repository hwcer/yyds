package player

// Locker 批量玩家锁的回调句柄(context.Mutex().Lock / Async 交给业务的那个对象)
//
// # 🔴 批量锁只保证「取得操作权限」,不保证玩家在线、也不保证有数据
//
// 这里拿到的 *Player 分两种,**用之前必须先判**:
//
//	p.Updater != nil   玩家本就在内存(在线,或离线未回收)。已 Reset,可以正常读写
//	p.Updater == nil   玩家不在内存(必然离线)。这是**空壳**,只占住了锁位没有任何数据,
//	                   直接拿它读写(cache.GetRole(p.Updater) 之类)就是当场 nil panic
//
// 空壳不是缺陷是设计:批量锁的典型用途是"改对方一两个字段、给对方发条消息",
// 而 Loading 会把该玩家**全部常驻模型整表拉一遍**(本项目 6 张表,含整个背包),
// 还把这份存档长期留在内存(回收要等缓存总量越过 MemoryPlayer+MemoryRelease)。
// 为一个字段付这份钱不合算,所以锁归锁、数据归数据。
//
// # 需要数据时按需手动加载,两条路按代价选
//
//	p.Initialize()   = Loading + Reset,幂等。要走 Updater 流水线(算子、溢出检查、
//	                   下发客户端)时用。⚠ 代价就是上面那份全量加载,改一两个字段别用它
//	直连 DB 改字段     空壳已经把并发的 Get / Load / 登录挡在门外——互斥性正是空壳给的保证。
//	                   ⚠ 写必须发生在**锁内**(回调返回前),否则与并发登录的 Loading 抢顺序
//
// # 空壳会自愈,不必也不该手动清理
//
// 空壳留在 Manage 里等人来补:登录走 Players.Load(init=true) / Login / Connected,
// 它们见 Updater==nil 就 New 一个并加载。期间别人可能已经拿到同一个指针在等锁,
// 删掉它等于让人在 Manage 之外操作孤儿对象。没人来就由 daemon 正常回收。
//
// # 下面几个聚合方法对空壳一律跳过
//
// Select / Data / Verify / Submit 遇到空壳直接 continue:它没有 Updater,
// 也就没有待拉取的 key、没有待提交的操作。跳过是正确语义,不是掩盖错误。
type Locker interface {
	Get(uid string) *Player
	Data() error
	Range(f func(player *Player) bool)
	Select(keys ...any)
	Verify() error
	Submit() error
}

type AsyncHandle func(locker Locker, args any)

type LockerHandle func(locker Locker, args any) (any, error)

// Reset 空壳(Updater==nil,见 players.Lock)上是空操作:没有数据可重置。
// 与 Release/Destroy 一起构成空壳流经全部路径的收敛点——Get/跨玩家 Locker/daemon 回收/
// 停服保存都经由这三个方法碰 Updater,守在这里一处顶五处。
func (p *Player) Reset() {
	if p.Updater == nil {
		return
	}
	p.Updater.Reset()
}

// Release 同 Reset,空壳上是空操作
func (p *Player) Release() {
	if p.Updater == nil {
		return
	}
	p.Updater.Release()
}

// Lock 取得玩家锁。加锁契约见 Player 的类型注释
//
// ⚠ 不可重入(sync.Mutex),而且没有超时:持锁期间再对**自己**调一次 players.Get 就是自锁死。
func (p *Player) Lock() {
	p.mutex.Lock()
}

func (p *Player) Unlock() {
	p.mutex.Unlock()
}

