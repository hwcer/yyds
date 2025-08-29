package players

import (
	"fmt"

	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
)

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
