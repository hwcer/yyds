package rooms

import (
	"github.com/hwcer/cosgo/session"
	"sync"
)

const (
	SessionPlayerRoomsName = "player.rooms"
)

var rooms = sync.Map{}

func Get(name string) (r *Room) {
	if i, ok := rooms.Load(name); ok {
		r = i.(*Room)
	}
	return
}

// All 所有房间
//func All(p *session.Data) (r map[string]struct{}) {
//	r = make(map[string]struct{})
//	if i := p.Get(SessionPlayerRoomsName); i != nil {
//		for k, v := range i.(map[string]struct{}) {
//			r[k] = v
//		}
//	}
//	return
//}

func loadOrCreate(name string, fixed bool) (r *Room, loaded bool) {
	room := NewRoom(name, fixed)
	var i any
	if i, loaded = rooms.LoadOrStore(name, room); loaded {
		r = i.(*Room)
	}
	return
}

func Join(name string, p *session.Data) {
	uuid := p.UUID()

	room, _ := loadOrCreate(name, false)
	room.Join(p)

	pms := Players.Load(uuid)
	if !pms.Has(name) {
		pms.Set(name, room)
	}
}

func Leave(name string, p *session.Data) {
	uuid := p.UUID()
	if pms := Players.Get(uuid); pms != nil {
		pms.Delete(name)
	}
	if room := Get(name); room != nil {
		room.Leave(p)
	}
}
func Range(name string, f func(*session.Data) bool) {
	room := Get(name)
	if room == nil {
		return
	}
	room.Range(f)
}

// Release 用户掉线时？销毁时清理所在房间信息
func Release(p *session.Data) {
	pms := Players.Delete(p.UUID())
	if pms == nil {
		return
	}
	for _, room := range pms.dict {
		room.Leave(p)
	}

}

// Delete 销毁房间
func Delete(name string) {
	i, loaded := rooms.LoadAndDelete(name)
	if !loaded {
		return
	}
	room := i.(*Room)
	room.Release()
}
