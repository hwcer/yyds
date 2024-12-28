package players

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosnet"
	"sync"
)

const (
	SessionPlayerSocketName = "player.sock"
)

type loginCallback func(player *session.Data, loaded bool) error

type players struct {
	sync.Map
}

// replace 顶号
func (this *players) replace(p *session.Data, socket *cosnet.Socket) {
	old := this.Socket(p)
	p.Set(SessionPlayerSocketName, socket, true)
	if old == nil || old.Id() == socket.Id() {
		return
	}
	old.Close()
	return
}

func (this *players) Socket(p *session.Data) *cosnet.Socket {
	i := p.Get(SessionPlayerSocketName)
	if i == nil {
		return nil
	}
	r, _ := i.(*cosnet.Socket)
	return r
}

func (this *players) Get(uuid string) *session.Data {
	v, ok := this.Load(uuid)
	if !ok {
		return nil
	}
	p, _ := v.(*session.Data)
	return p
}

func (this *players) Range(fn func(*session.Data) bool) {
	this.Map.Range(func(k, v interface{}) bool {
		if p, ok := v.(*session.Data); ok {
			return fn(p)
		}
		return true
	})
}

func (this *players) Delete(p *session.Data) bool {
	if p == nil {
		return false
	}
	this.Map.Delete(p.UUID())
	sock := this.Socket(p)
	if sock != nil {
		sock.Close()
	}
	return true
}

func (this *players) Login(p *session.Data, callback loginCallback) (err error) {
	p.Lock()
	defer p.Unlock()
	r := p
	i, loaded := this.Map.LoadOrStore(p.UUID(), p)
	if loaded {
		sp, _ := i.(*session.Data)
		sp.Lock()
		defer sp.Unlock()
		sp.Reset()
		sp.Merge(p, true)
		r = sp
	}
	if callback != nil {
		err = callback(r, loaded)
	}
	return
}

// Binding 身份认证绑定socket
func (this *players) Binding(socket *cosnet.Socket, uuid string, data map[string]any) (r *session.Data, err error) {
	p := session.NewData(uuid, "", data)
	err = this.Login(p, func(player *session.Data, loaded bool) error {
		if loaded {
			this.replace(player, socket)
		} else {
			player.Set(SessionPlayerSocketName, socket, true)
		}
		r = player
		return nil
	})
	if err == nil {
		socket.OAuth(r)
	}
	return
}
