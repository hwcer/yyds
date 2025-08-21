package player

import "sync"

type Manage struct {
	players map[string]*Player
	mutex   sync.RWMutex
}

func NewManage() *Manage {
	return &Manage{players: make(map[string]*Player)}
}

func (m *Manage) Load(key string) (value *Player, ok bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	value, ok = m.players[key]
	return
}

func (m *Manage) Range(f func(key string, value *Player) bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	for k, v := range m.players {
		if !f(k, v) {
			break
		}
	}
}

func (m *Manage) Store(key string, value *Player) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.players[key] = value

}

func (m *Manage) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.players = make(map[string]*Player)
}
func (m *Manage) Delete(key string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.players, key)
}

func (m *Manage) LoadOrStore(key string, value *Player) (actual *Player, loaded bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if actual, loaded = m.players[key]; !loaded {
		m.players[key] = value
		actual = value
	}
	return
}

func (m *Manage) LoadAndDelete(key string) (value *Player, loaded bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if value, loaded = m.players[key]; loaded {
		delete(m.players, key)
	}
	return
}
