package player

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

func (p *Player) Lock() {
	p.Syncer.Lock()
}

func (p *Player) Unlock() {
	p.Syncer.Unlock()
}

