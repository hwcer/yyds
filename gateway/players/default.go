package players

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosnet"
	"sync"
)

var Players = players{Map: sync.Map{}}

func Socket(p *session.Data) *cosnet.Socket {
	return Players.Socket(p)
}

func Get(uuid string) *session.Data {
	return Players.Get(uuid)
}

func Range(fn func(*session.Data) bool) {
	Players.Range(fn)
}

func Delete(p *session.Data) bool {
	return Players.Delete(p)
}

func Login(p *session.Data, callback loginCallback) (err error) {
	return Players.Login(p, callback)
}

// Binding 身份认证绑定socket
func Binding(socket *cosnet.Socket, uuid string, data map[string]any) (r *session.Data, err error) {
	return Players.Binding(socket, uuid, data)
}
