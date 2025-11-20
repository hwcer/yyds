package players

import (
	"strings"

	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
)

func Socket(p *session.Data) *cosnet.Socket {
	i := p.Get(SessionPlayerSocketName)
	if i == nil {
		return nil
	}
	r, _ := i.(*cosnet.Socket)
	return r
}

// Replace  长连接顶号,也可能是被短连接顶掉线（sock==nil）
func Replace(p *session.Data, sock *cosnet.Socket, ip string) {
	os := Socket(p)
	p.Mutex(func(setter session.Setter) {
		var reconnect bool
		if os != nil && (sock == nil || os.Id() != sock.Id()) {
			if i := strings.Index(ip, ":"); i > 0 {
				ip = ip[:i]
			}
			reconnect = true
			os.Replaced(ip)
		}
		if sock != nil {
			setter.Set(SessionPlayerSocketName, sock)
			sock.Authentication(p, reconnect)
		}
	})
	return
}

func Connect(sock *cosnet.Socket, guid string, value values.Values) (data *session.Data, err error) {
	if _, data, err = Login(guid, value); err == nil {
		Replace(data, sock, sock.RemoteAddr().String())
	}
	return
}
func Reconnect(sock *cosnet.Socket, secret string) (data *session.Data, err error) {
	if v := sock.Data(); v != nil {
		return
	}
	s := session.New()
	if err = s.Verify(secret); err != nil {
		return
	}
	_, err = s.Refresh() //刷线TOKEN
	data = s.Data
	Replace(data, sock, sock.RemoteAddr().String())
	return
}

func Disconnect(sock *cosnet.Socket) (err error) {
	data := sock.Data()
	if data == nil {
		return
	}
	os := Socket(data)
	data.Mutex(func(setter session.Setter) {
		if os != nil && os.Id() == sock.Id() {
			setter.Delete(SessionPlayerSocketName)
		}
	})
	return
}
