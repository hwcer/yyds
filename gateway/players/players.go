package players

import (
	"fmt"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
	"strings"
	"sync"
)

const (
	SessionPlayerSocketName = "player.sock"
)

type loginCallback func(player *session.Data, loaded bool) error

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

func (this *players) Login(guid string, value values.Values, callback loginCallback) (err error) {
	r := session.NewData(guid, value)
	r.Lock()
	defer r.Unlock()
	i, loaded := this.Map.LoadOrStore(guid, r)
	if loaded {
		p, _ := i.(*session.Data)
		p.Lock()
		defer p.Unlock()
		p.Merge(r, true)
		r = p
	} else {
		err = session.Options.Storage.New(r)
	}
	if callback != nil {
		err = callback(r, loaded)
	}
	return
}

func (this *players) Connect(sock *cosnet.Socket, guid string, value values.Values) error {
	err := this.Login(guid, value, func(data *session.Data, loaded bool) error {
		if loaded {
			this.replace(data, sock)
		}
		data.Set(SessionPlayerSocketName, sock, true)
		sock.OAuth(data)
		return nil
	})
	return err
}

func (this *players) Disconnect(sock *cosnet.Socket) (err error) {
	i := sock.Data()
	if i == nil {
		return
	}
	data, ok := i.(*session.Data)
	if !ok {
		return fmt.Errorf("socket data type error:%v", i)
	}

	data.Lock()
	defer data.Unlock()
	if s := this.Socket(data); s != nil && s.Id() == sock.Id() {
		data.Delete(SessionPlayerSocketName, true)
	}
	return
}

func (this *players) Reconnect(sock *cosnet.Socket, secret string) (data *session.Data, err error) {
	if v := sock.Data(); v != nil {
		return
	}
	s := session.New()
	if err = s.Verify(secret); err != nil {
		return
	}
	data = s.Data
	this.replace(data, sock)
	data.Set(SessionPlayerSocketName, sock)
	sock.Reconnect(data)
	return
}
