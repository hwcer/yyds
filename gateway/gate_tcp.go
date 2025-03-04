package gateway

import (
	"bytes"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
	"github.com/hwcer/cosnet/tcp"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
	"net"
	"strconv"
	"time"
)

type Socket struct {
}

func (this *Socket) init() error {
	if !cosnet.Start() {
		return nil
	}
	service := cosnet.Service("")
	_ = service.Register(this.proxy, "/*")
	cosnet.On(cosnet.EventTypeError, this.Errorf)
	//cosnet.On(cosnet.EventTypeConnected, this.Connected)
	//cosnet.On(cosnet.EventTypeDisconnect, this.Disconnect)
	return nil
}

func (this *Socket) Listen(address string) error {
	_, err := cosnet.Listen(address)
	if err == nil {
		logger.Trace("网关长连接启动：%v", options.Gate.Address)
	}
	return err
}

func (this *Socket) Accept(ln net.Listener) error {
	cosnet.Accept(&tcp.Listener{Listener: ln})
	logger.Trace("网关长连接启动：%v", options.Gate.Address)
	return nil
}

func (this *Socket) Errorf(socket *cosnet.Socket, err interface{}) {
	logger.Alert(err)
}

func (this *Socket) ping(c *cosnet.Context) interface{} {
	var s string
	_ = c.Bind(&s)
	return []byte(strconv.Itoa(int(time.Now().Unix())))
}

func (this *Socket) proxy(c *cosnet.Context) interface{} {
	h := &socketProxy{Context: c}
	reply, err := proxy(h)
	if err != nil {
		return c.Errorf(0, err)
	}
	return reply
}

//func (this *Socket) Connected(sock *cosnet.Socket, _ interface{}) {
//	logger.Debug("Connected:%v", sock.Id())
//}
//
//func (this *Socket) Disconnect(sock *cosnet.Socket, _ interface{}) {
//	logger.Debug("Disconnect:%v", sock.Id())
//}

type socketProxy struct {
	*cosnet.Context
}

func (this *socketProxy) Data() (*session.Data, error) {
	return this.Context.Socket.Data(), nil
}

func (this *socketProxy) Buffer() (buf *bytes.Buffer, err error) {
	buff := bytes.NewBuffer(this.Context.Message.Body())
	return buff, nil
}
func (this *socketProxy) Login(guid string, value values.Values) (err error) {
	_, err = players.Binding(this.Context.Socket, guid, value)
	return
}

func (this *socketProxy) Delete() error {
	this.Context.Socket.Close()
	return nil
}

func (this *socketProxy) Metadata() values.Metadata {
	q := this.Message.Query()
	if _, ok := q[binder.HeaderContentType]; !ok {
		q[binder.HeaderContentType] = options.Options.Binder
	}
	return q
}
