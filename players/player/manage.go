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
	//🔴 只持读锁做快照,回调在锁外跑:daemon 的扫描回调会做重活(状态判定/收集),
	//持管理器读锁跑慢回调会把登录路径(写锁)整体挡住。容量必须 0 起——
	//make([]*Player, len) 造出 N 个 nil 再 append,前半段全是 nil,回调当场解引用崩溃
	m.mutex.RLock()
	ps := make([]*Player, 0, len(m.players))
	for _, v := range m.players {
		ps = append(ps, v)
	}
	m.mutex.RUnlock()
	for _, v := range ps {
		if !f(v.Key(), v) {
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
