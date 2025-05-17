package player

type Locker interface {
	Data() error
	Get(uid string) *Player
	Range(f func(player *Player) bool)
	Select(keys ...any)
	Verify() error
	Submit() error
}

type LockerHandle func(locker Locker) (any, error)

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
