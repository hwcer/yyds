package gateway

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
	"github.com/hwcer/cosnet/tcp"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
)

func NewTCPServer() *TcpServer {
	s := &TcpServer{}
	return s
}

type TcpServer struct {
	//Errorf func(*cosnet.Context, error) any
}

func (this *TcpServer) init() error {
	if !cosnet.Start() {
		return nil
	}
	//关闭 cosnet 计时器,由session接管
	cosnet.Options.Heartbeat = 0
	session.Heartbeat.On(cosnet.Heartbeat)

	service := cosnet.Service("")
	_ = service.Register(this.proxy, "/*")
	return nil
}

func (this *TcpServer) Listen(address string) error {
	_, err := cosnet.Listen(address)
	if err == nil {
		logger.Trace("网关长连接启动：%v", options.Gate.Address)
	}
	return err
}

func (this *TcpServer) Accept(ln net.Listener) error {
	cosnet.Accept(&tcp.Listener{Listener: ln})
	logger.Trace("网关长连接启动：%v", options.Gate.Address)
	return nil
}

func (this *TcpServer) proxy(c *cosnet.Context) error {
	h := &socketProxy{Context: c}
	b, err := proxy(h)
	if err != nil {
		return err
	}
	if c.Message.Confirm() {
		return c.Reply(b)
	}
	return nil
}

type socketProxy struct {
	*cosnet.Context
}

func (this *socketProxy) Path() (string, error) {
	r, _, err := this.Context.Path()
	return r, err
}
func (this *socketProxy) Data() (*session.Data, error) {
	i := this.Context.Socket.Data()
	if i == nil {
		return nil, nil
	}
	v, _ := i.(*session.Data)
	return v, nil
}
func (this *socketProxy) Socket() *cosnet.Socket {
	return this.Context.Socket
}
func (this *socketProxy) Buffer() (buf *bytes.Buffer, err error) {
	buff := bytes.NewBuffer(this.Context.Message.Body())
	return buff, nil
}
func (this *socketProxy) Login(guid string, value values.Values) (err error) {
	if i := this.Context.Socket.Data(); i != nil {
		v, _ := i.(*session.Data)
		if v.UUID() == guid {
			return nil
		} else {
			return errors.New("please do not login again")
		}
	}
	return players.Connect(this.Context.Socket, guid, value)
}

func (this *socketProxy) Delete() error {
	this.Context.Socket.Close()
	return nil
}

func (this *socketProxy) Metadata() values.Metadata {
	meta := values.Metadata{}
	if _, q, _ := this.Context.Path(); q != "" {
		query, _ := url.ParseQuery(q)
		for k, _ := range query {
			meta[k] = query.Get(k)
		}
	}
	magic := this.Message.Magic()
	meta[binder.HeaderContentType] = magic.Binder.Name()
	meta[options.ServiceMetadataRequestId] = fmt.Sprintf("%d", this.Context.Message.Index())
	return meta
}
