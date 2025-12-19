package players

import (
	"sync"

	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
)

const (
	SessionPlayerSocketName = "player.sock"
)

var players = sync.Map{}

func Get(uuid string) *session.Data {
	v, ok := players.Load(uuid)
	if !ok {
		return nil
	}
	p, _ := v.(*session.Data)
	return p
}

func Range(fn func(*session.Data) bool) {
	players.Range(func(k, v interface{}) bool {
		if p, ok := v.(*session.Data); ok {
			return fn(p)
		}
		return true
	})
}

func Delete(p *session.Data) bool {
	if p == nil {
		return false
	}
	players.Delete(p.UUID())
	sock := Socket(p)
	if sock != nil {
		sock.Close()
	}
	return true
}

func Login(guid string, value values.Values) (token string, data *session.Data, err error) {
	data = session.NewData(guid, value)
	i, loaded := players.LoadOrStore(guid, data)
	if loaded {
		p, _ := i.(*session.Data)
		p.Update(value)
		data = p
	}
	ss := session.New(data)
	if !loaded {
		token, err = ss.New(data)
	} else {
		token, err = ss.Refresh() //刷新TOKEN 强制其他TOKEN失效
	}
	return
}
