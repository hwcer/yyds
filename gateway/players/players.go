package players

import (
	"errors"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosnet"
	"strings"
	"sync"
)

const (
	SessionPlayerSocketName = "player.sock"
)

type loginCallback func(player *session.Data, old *session.Data) error

type players struct {
	sync.Map
}

// Replace  长连接顶号
func (this *players) replace(p *session.Data, socket *cosnet.Socket) {
	os := this.Socket(p)
	if os != nil && os.Id() != socket.Id() {
		ip := socket.RemoteAddr().String()
		if i := strings.Index(ip, ":"); i > 0 {
			ip = ip[:i]
		}
		os.Replaced(ip)
	}
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
	var old *session.Data
	defer func() {
		if old != nil {
			_ = session.Options.Storage.Delete(old)
		}
	}()
	p.Lock()
	defer p.Unlock()
	if i, loaded := this.Map.LoadOrStore(p.UUID(), p); loaded {
		old, _ = i.(*session.Data)
		old.Lock()
		defer old.Unlock()
		p.Merge(old, true)
	}
	if callback != nil {
		err = callback(p, old)
	}
	return
}
func (this *players) Connect(sock *cosnet.Socket, v *session.Data) error {
	err := this.Login(v, func(data *session.Data, old *session.Data) error {
		if old != nil {
			this.replace(old, sock)
		}
		data.Set(SessionPlayerSocketName, sock, true)
		sock.OAuth(data)
		return nil
	})
	return err
}

func (this *players) Reconnect(sock *cosnet.Socket, secret string) (err error) {
	if v := sock.Data(); v != nil {
		return errors.New("please do not login again")
	}
	s := session.New()
	if err = s.Verify(secret); err != nil {
		return
	}
	this.replace(s.Data, sock)
	s.Data.Set(SessionPlayerSocketName, sock, true)
	sock.Reconnect(s.Data)
	return
}
