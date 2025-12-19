package channel

import (
	"github.com/hwcer/cosgo/session"
	"sync"
)

var manage = sync.Map{}

func Get(name string) (r *Channel) {
	if i, ok := manage.Load(name); ok {
		r = i.(*Channel)
	}
	return
}

func loadOrCreate(name string, fixed bool) (r *Channel, loaded bool) {
	r = New(name, fixed)
	var i any
	if i, loaded = manage.LoadOrStore(name, r); loaded {
		r = i.(*Channel)
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

// Release 用户掉线,销毁时 清理所在房间信息
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
	i, loaded := manage.LoadAndDelete(name)
	if !loaded {
		return
	}
	room := i.(*Channel)
	room.Release()
}
