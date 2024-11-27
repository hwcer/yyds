package player

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
