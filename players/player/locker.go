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

func (p *Player) Reset() {
	p.Updater.Reset()
}

func (p *Player) Release() {
	p.Updater.Release()
}

func (p *Player) Lock() {
	p.mutex.Lock()
}

func (p *Player) Unlock() {
	p.mutex.Unlock()
}
func (p *Player) TryLock() bool {
	return p.mutex.TryLock()
}
