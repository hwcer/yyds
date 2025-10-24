package gateway

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

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
	//关闭 cosnet 计时器,由session接管
	cosnet.Options.Heartbeat = 0
	session.Heartbeat.On(cosnet.Heartbeat)

	service := cosnet.Service()
	_ = service.Register(this.proxy, "*")
	_ = service.Register(this.oauth, Options.OAuth)
	h := service.Handler().(*cosnet.Handler)
	h.SetSerialize(func(c *cosnet.Context, reply any) ([]byte, error) {
		return Options.Serialize(c, reply)
	})
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
func (this *TcpServer) oauth(c *cosnet.Context) any {
	var err error
	authorize := &Authorize{}
	if err = c.Bind(&authorize); err != nil {
		return err
	}
	token, err := authorize.Verify()
	if err != nil {
		return err
	}
	h := socketProxy{Context: c}
	vs := values.Values{}
	if token.Developer {
		vs.Set(options.ServiceMetadataDeveloper, "1")
	}
	if err = h.Login(token.Guid, vs); err != nil {
		return err
	}
	var r any
	if r, err = oauth(&h); err != nil {
		return err
	}
	return r
}
func (this *TcpServer) proxy(c *cosnet.Context) any {
	h := &socketProxy{Context: c}
	p, err := h.Path()
	if err != nil {
		return err
	}
	var b []byte
	if b, err = caller(h, p); err != nil {
		return err
	} else {
		return b
	}
}

type socketProxy struct {
	*cosnet.Context
}

func (this *socketProxy) Path() (string, error) {
	r, _, err := this.Context.Path()
	return r, err
}
func (this *socketProxy) Cookie() (*session.Data, error) {
	data := this.Context.Socket.Data()
	if data == nil {
		return nil, session.ErrorSessionNotExist
	}
	return data, nil
}

func (this *socketProxy) Login(guid string, value values.Values) (err error) {
	if v := this.Context.Socket.Data(); v != nil {
		if v.UUID() == guid {
			return nil
		} else {
			return errors.New("please do not login again")
		}
	}
	return players.Connect(this.Context.Socket, guid, value)
}

func (this *socketProxy) Logout() error {
	this.Context.Socket.Close()
	return nil
}

func (this *socketProxy) Socket() *cosnet.Socket {
	return this.Context.Socket
}
func (this *socketProxy) Buffer() (buf *bytes.Buffer, err error) {
	buff := bytes.NewBuffer(this.Context.Message.Body())
	return buff, nil
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

func (this *socketProxy) RemoteAddr() string {
	ip := this.Context.RemoteAddr().String()
	if i := strings.Index(ip, ":"); i > 0 {
		ip = ip[0:i]
	}
	return ip
}
