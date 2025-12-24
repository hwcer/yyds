package channel

import (
	"sync"

	"github.com/hwcer/cosgo/session"
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

func Join(p *session.Data, name string) {
	k, v := Split(name)
	setter := NewSetter(p)
	if old, ok := setter.Join(k, v); ok && old != v {
		leave(p, name, old)
	}
	room, _ := loadOrCreate(name, false)
	room.Join(p)

}

func Leave(p *session.Data, name string) {
	k, v := Split(name)
	setter := NewSetter(p)
	setter.Leave(k, v)
	leave(p, k, v)
}

func leave(p *session.Data, k, v string) {
	name := Name(k, v)
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
	setter := NewSetter(p)
	rs := setter.Release()
	for _, r := range rs {
		leave(p, r.k, r.v)
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
