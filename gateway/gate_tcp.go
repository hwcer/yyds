package gateway

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
	"github.com/hwcer/cosnet/tcp"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
	"net"
	"net/url"
	"strconv"
	"time"
)

type Socket struct {
	Errorf func(*cosnet.Context, error) any
}

func (this *Socket) init() error {
	if !cosnet.Start() {
		return nil
	}
	service := cosnet.Service("")
	_ = service.Register(this.proxy, "/*")
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

func (this *Socket) ping(c *cosnet.Context) interface{} {
	var s string
	_ = c.Bind(&s)
	return []byte(strconv.Itoa(int(time.Now().Unix())))
}

func (this *Socket) proxy(c *cosnet.Context) (r any) {
	h := &socketProxy{Context: c}
	var err error
	if r, err = proxy(h); err != nil {
		r = this.errorf(c, err)
	}
	return
}
func (this *Socket) errorf(c *cosnet.Context, err error) any {
	if this.Errorf != nil {
		return this.Errorf(c, err)
	}
	return values.Error(err)
}

type socketProxy struct {
	*cosnet.Context
}

func (this *socketProxy) Path() (string, error) {
	r, _, err := this.Context.Path()
	return r, err
}
func (this *socketProxy) Data() (*session.Data, error) {
	return this.Context.Socket.Data(), nil
}

func (this *socketProxy) Buffer() (buf *bytes.Buffer, err error) {
	buff := bytes.NewBuffer(this.Context.Message.Body())
	return buff, nil
}
func (this *socketProxy) Login(sess *session.Session) (err error) {
	if v := this.Socket.Data(); v != nil {
		if v.UUID() == sess.UUID() {
			return nil
		} else {
			return errors.New("please do not login again")
		}
	}
	return players.Connect(this.Context.Socket, sess.Data)
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
