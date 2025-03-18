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

// Connect 长连接登陆
func Connect(socket *cosnet.Socket, v *session.Data) error {
	return Players.Connect(socket, v)
}

// Reconnect 长连接断线重连
func Reconnect(sock *cosnet.Socket, secret string) error {
	return Players.Reconnect(sock, secret)
}
