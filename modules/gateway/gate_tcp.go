package gateway

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
	"github.com/hwcer/cosnet/tcp"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/modules/gateway/players"
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

	cosnet.On(cosnet.EventTypeReplaced, this.S2CReplaced)
	cosnet.On(cosnet.EventTypeDisconnect, this.Disconnect)
	cosnet.On(cosnet.EventTypeAuthentication, this.S2CSecret)

	service := cosnet.Service()
	_ = service.Register(this.proxy, "*")
	_ = service.Register(this.C2SPing, "ping")
	_ = service.Register(this.C2SOAuth, Options.C2SOAuth)
	_ = service.Register(this.C2SReconnect, "C2SReconnect")

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

func (this *TcpServer) C2SPing(c *cosnet.Context) any {
	ms := time.Now().UnixMilli()
	s := strconv.Itoa(int(ms))
	return []byte(s)
}

func (this *TcpServer) C2SOAuth(c *cosnet.Context) any {
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
	} else {
		vs.Set(options.ServiceMetadataDeveloper, "")
	}
	if _, err = h.Login(token.Guid, vs); err != nil {
		return err
	}
	var r any
	if r, err = oauth(&h); err != nil {
		return err
	}
	return r
}

// S2CSecret  默认的发送断线重连密钥
// cosnet.On(cosnet.EventTypeAuthentication, S2CSecret)
func (this *TcpServer) S2CSecret(sock *cosnet.Socket, _ any) {
	data := sock.Data()
	if data == nil {
		return
	}
	ss := session.New(data)
	if token, err := ss.Token(); err != nil {
		sock.Errorf(err)
	} else if Options.S2CSecret != nil {
		Options.S2CSecret(sock, token)
	} else {
		sock.Send(0, "S2CSecret", []byte(token))
	}
	return
}

// S2CReplaced  默认的顶号提示
func (this *TcpServer) S2CReplaced(sock *cosnet.Socket, i any) {
	if sock == nil {
		return
	}
	ip, ok := i.(string)
	if !ok {
		return
	}
	if Options.S2CReplaced != nil {
		Options.S2CReplaced(sock, ip)
	} else {
		sock.Send(0, "S2CReplaced", []byte(ip))
	}
}
func (this *TcpServer) C2SReconnect(c *cosnet.Context) any {
	secret := string(c.Message.Body())
	if secret == "" {
		return errors.ErrArgEmpty
	}
	if _, err := players.Reconnect(c.Socket, secret); err != nil {
		return err
	}
	return true
}
func (this *TcpServer) Disconnect(sock *cosnet.Socket, _ any) {
	if err := players.Disconnect(sock); err != nil {
		logger.Alert("Disconnect error:%v", err)
	}
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

func (this *socketProxy) Login(guid string, value values.Values) (token string, err error) {
	data := this.Context.Socket.Data()
	if data != nil {
		if data.UUID() != guid {
			return "", errors.New("please do not login again")
		}
	} else if data, err = players.Connect(this.Context.Socket, guid, value); err != nil {
		return
	}
	ss := session.New(data)
	return ss.Token()
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
