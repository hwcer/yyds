package channel

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/logger"
	"sync"
)

func New(name string, fixed bool) *Channel {
	return &Channel{id: name, fixed: fixed, ps: map[string]*session.Data{}}
}

type Channel struct {
	id       string
	ps       map[string]*session.Data
	fixed    bool //固定频道不会自动删除
	locker   sync.Mutex
	released bool //已经删除 无法进入
}

func (this *Channel) Id() string {
	return this.id
}
func (this *Channel) Has(v *session.Data) bool {
	_, ok := this.ps[v.UUID()]
	return ok
}

func (this *Channel) Join(d *session.Data) bool {
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

func (this *Channel) Leave(d *session.Data) bool {
	if !this.Has(d) {
		return false
	}
	this.locker.Lock()
	defer this.locker.Unlock()
	delete(this.ps, d.UUID())
	if !this.fixed && len(this.ps) == 0 {
		this.released = true
		manage.Delete(this.id)
		logger.Debug("人数为空，房间销毁:%s", this.id)
	}
	return true
}

func (this *Channel) Release() {
	this.locker.Lock()
	defer this.locker.Unlock()
	this.released = true
	this.removeAllPlayer()
	//manage.Delete(this.id)
}

// release 房间销毁时，清理所有房间内的成员
func (this *Channel) removeAllPlayer() {
	PMSMutex.Lock()
	defer PMSMutex.Unlock()
	for _, v := range this.ps {
		uuid := v.UUID()
		if pms := Players.Get(uuid); pms != nil {
			pms.remove(this.id)
		}
	}
}

func (this *Channel) Range(f func(*session.Data) bool) {
	for _, p := range this.ps {
		if !f(p) {
			return
		}
	}
}

func (this *Channel) Broadcast(path string, data []byte) {
	this.Range(func(p *session.Data) bool {
		SendMessage(p, path, data)
		return true
	})
}
