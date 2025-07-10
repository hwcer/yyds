package rooms

import (
	"github.com/hwcer/cosgo/session"
	ps "github.com/hwcer/yyds/gateway/players"
	"sync"
)

func NewRoom(name string, fixed bool) *Room {
	return &Room{id: name, fixed: fixed, ps: map[string]*session.Data{}}
}

type Room struct {
	id       string
	ps       map[string]*session.Data
	fixed    bool //固定频道不会自动删除
	locker   sync.Mutex
	released bool //已经删除 无法进入
}

func (this *Room) Id() string {
	return this.id
}
func (this *Room) Has(v *session.Data) bool {
	_, ok := this.ps[v.UUID()]
	return ok
}

func (this *Room) Join(d *session.Data) bool {
	if this.Has(d) {
		return true
	}
	this.locker.Lock()
	defer this.locker.Unlock()
	if this.released {
		return false
	}

	vs := map[string]*session.Data{}
	for k, v := range this.ps {
		vs[k] = v
	}
	vs[d.UUID()] = d
	this.ps = vs
	return true
}

func (this *Room) Leave(d *session.Data) bool {
	if !this.Has(d) {
		return false
	}
	this.locker.Lock()
	defer this.locker.Unlock()
	delete(this.ps, d.UUID())
	if !this.fixed && len(this.ps) == 0 {
		this.released = true
		rooms.Delete(this.id)
	}
	return true
}

func (this *Room) Release() {
	this.locker.Lock()
	defer this.locker.Unlock()
	this.released = true
	this.removeAllPlayer()
	//rooms.Delete(this.id)
}

// release 房间销毁时，清理所有房间内的成员
func (this *Room) removeAllPlayer() {
	PMSMutex.Lock()
	defer PMSMutex.Unlock()
	for _, v := range this.ps {
		uuid := v.UUID()
		if pms := Players.Get(uuid); pms != nil {
			pms.remove(this.id)
		}
	}
}

func (this *Room) Range(f func(*session.Data) bool) {
	for _, p := range this.ps {
		if !f(p) {
			return
		}
	}
}

func (this *Room) Broadcast(path string, data []byte) {
	this.Range(func(p *session.Data) bool {
		if sock := ps.Socket(p); sock != nil {
			_ = sock.Send(0, path, data)
		}
		return true
	})
}
