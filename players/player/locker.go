package player

// Locker 批量玩家锁的回调句柄(context.Mutex().Lock / Async 交给业务的那个对象)
//
// # 🔴 批量锁兼容离线操作:每个玩家都有 Updater,但**读玩家数据时要假设玩家不在线**
//
// 回调里拿到的每个 *Player 都有可用的 p.Updater(已 Reset),不再有空壳:
//
//	玩家本就在内存   用现成的(在线,或离线未回收),数据完整
//	玩家不在内存     批量锁以 updater **懒加载**给他建 Updater(Loading(lazy=true)):不预拉任何数据,
//	                 不付「整档六张表全量加载 + 长期驻留」的代价
//
// 业务**不要**去分辨是哪一种(在线与否会在取锁后、回调前变化),一律按「对方不在线、懒加载」写:
//
//	读   读之前必须 Select + Data(locker.Select + locker.Data 或 p.Updater 上逐句柄 Select);
//	     没拉的键读到**零值**——「读出来判定再写绝对值」漏了这一步,就会用零值覆盖对方存档
//	写   照常 Add / Sub / Set,写会自己 Select 触碰的键;结束前 locker.Submit()(Async 必须显式提交)
//	扫描 Range / Len / All / 不可叠加道具的 Count 懒加载下报错(u.Error),不要在批量锁里用
//	派生 依赖 EventTypeLoaded 建的派生状态(项目的物品索引、域级缓存里的统计)懒加载下为空,
//	     不要依赖;需要整档数据的逻辑不要放进批量锁
//
// 只改一两个顶层标量、且不读任何数据时,锁内直连 DB(UpdateMany,读零次)仍是最省的,
// 判据用 !p.Updater.Loaded()(懒加载);⚠ 写必须发生在**锁内**,否则与并发登录的 Loading 抢顺序。
//
// 懒加载玩家留在 Manage 里:之后登录(Players.Load(lazy=false) / Login / Connected)由 Loading
// 自动转正为整档在线;Players.Get 把未转正的视为不在线(ErrNotOnline),走「Get 失败 → Load」
// 降级链完成转正。没人来就由 daemon 正常回收。
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

// Reset Updater==nil 时是空操作。经 Get / Load / 批量锁交给业务的玩家一定有 Updater(不再有空壳),
// nil 只剩新对象在 acquire 之后、prepare 之前那一瞬(锁内)与 Unload 之后——判 nil 是防御:
// Get/跨玩家 Locker/daemon 回收/停服保存都经由 Reset/Release/Unload 碰 Updater,守在这里一处顶五处。
func (p *Player) Reset() {
	if p.Updater == nil {
		return
	}
	p.Updater.Reset()
}

// Release 同 Reset,Updater==nil 时是空操作
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

