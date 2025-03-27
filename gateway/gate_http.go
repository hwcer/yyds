package gateway

import (
	"errors"
	"fmt"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosweb"
	"github.com/hwcer/cosweb/middleware"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var Method = []string{"POST", "GET", "OPTIONS"}

func NewHttpServer() *HttpServer {
	s := &HttpServer{}
	return s
}

type HttpServer struct {
	*cosweb.Server
	redis any //是否使用redis存储session信息
}

func (this *HttpServer) init() (err error) {
	this.Server = cosweb.New()
	//跨域
	access := middleware.NewAccessControlAllow()
	access.Origin("*")
	access.Methods(Method...)
	headers := []string{session.Options.Name, "Accept", "Content-Type", "Set-Cookie", "X-Forwarded-Key", "X-Forwarded-Val"}
	access.Headers(strings.Join(headers, ","))
	this.Server.Use(access.Handle)
	this.Server.Register("/*", this.proxy, Method...)

	if options.Gate.Static != nil && options.Gate.Static.Root != "" {
		static := this.Server.Static(options.Gate.Static.Route, options.Gate.Static.Root)
		if options.Gate.Static.Index != "" {
			static.Index(options.Gate.Static.Index)
		}
	}
	return nil
}

func (this *HttpServer) Listen(address string) (err error) {
	if err = this.Server.Listen(address); err == nil {
		logger.Trace("网关短连接启动：%v", options.Gate.Address)
	}
	return
}
func (this *HttpServer) Accept(ln net.Listener) (err error) {
	if err = this.Server.Accept(ln); err == nil {
		logger.Trace("网关短连接启动：%v", options.Gate.Address)
	}
	return
}

func (this *HttpServer) proxy(c *cosweb.Context, next cosweb.Next) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = values.Errorf(0, r)
		}
	}()
	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > elapsedMillisecond {
			buff, _ := c.Buffer()
			logger.Alert("发现高延时请求,TIME:%v,PATH:%v,BODY:%v", elapsed, c.Request.URL.Path, string(buff.Bytes()))
		}
	}()
	h := &httpProxy{Context: c}
	reply, err := proxy(h)
	if err != nil {
		return err
	}
	if v := c.GetString(session.Options.Name, cosweb.RequestDataTypeContext); v != "" {
		s := string(reply)
		if strings.Contains(s, options.Cookies.Name) {
			s = strings.Replace(s, options.Cookies.Name, session.Options.Name, -1)
			s = strings.Replace(s, options.Cookies.Value, v, -1)
			reply = []byte(s)
		} else if strings.HasPrefix(s, "{") {
			sb := strings.Builder{}
			sb.WriteString("{")
			sb.WriteString(fmt.Sprintf(`"cookie":{"key":"%v","val":"%v"},`, session.Options.Name, v))
			sb.WriteString(s[1:])
			reply = []byte(sb.String())
		}
	}
	b := h.Binder()
	if b == nil {
		return errors.New("unknown accept content type")
	}
	return c.Bytes(cosweb.ContentType(b.String()), reply)
}

type httpProxy struct {
	*cosweb.Context
	uri      *url.URL
	metadata values.Metadata
}

func (this *httpProxy) Binder() binder.Binder {
	var t string
	meta := this.Metadata()
	if t = meta[binder.HeaderAccept]; t == "" {
		t = meta[binder.HeaderContentType]
	}
	return binder.Get(t)
}

//func (this *httpProxy) errorf(c *cosweb.Context, err error) error {
//	if mod.Server.Errorf != nil {
//		return mod.Server.Errorf(c, err)
//	}
//	data := values.Parse(err)
//	b := this.Binder()
//	if b == nil {
//		return err
//	}
//	s, err := b.Marshal(data)
//	if err != nil {
//		return err
//	}
//	return this.Context.Bytes(cosweb.ContentType(b.String()), s)
//}

func (this *httpProxy) Path() (string, error) {
	return this.Context.Request.URL.Path, nil
}

func (this *httpProxy) Data() (*session.Data, error) {
	token := this.Context.GetString(session.Options.Name, cosweb.RequestDataTypeCookie, cosweb.RequestDataTypeQuery, cosweb.RequestDataTypeHeader)
	if token == "" {
		return nil, values.Error("token empty")
	}
	if err := this.Context.Session.Verify(token); err != nil {
		return nil, err
	}
	return this.Context.Session.Data, nil
}

func (this *httpProxy) Login(sess *session.Session) (err error) {
	this.Context.Session = sess
	cookie := &http.Cookie{Name: session.Options.Name, Path: "/", Value: sess.Token()}
	err = players.Login(sess.Data, func(data *session.Data, _ bool) error {
		this.Context.Session.Data = data
		return nil
	})
	if err != nil {
		return err
	}
	http.SetCookie(this.Context.Response, cookie)
	header := this.Header()
	header.Set("X-Forwarded-Key", session.Options.Name)
	header.Set("X-Forwarded-Val", cookie.Value)
	this.Context.Set(session.Options.Name, cookie.Value)
	return nil
}

func (this *httpProxy) Delete() error {
	return this.Context.Session.Delete()
}

func (this *httpProxy) Metadata() values.Metadata {
	if this.metadata != nil {
		return this.metadata
	}
	this.metadata = make(values.Metadata)
	q := this.Context.Request.URL.Query()
	for k, _ := range q {
		this.metadata[k] = q.Get(k)
	}
	if t := this.ContentType(binder.HeaderContentType, ";"); t != "" {
		this.metadata.Set(binder.HeaderContentType, t)
	} else {
		this.metadata.Set(binder.HeaderContentType, options.Options.Binder)
	}
	if t := this.ContentType(binder.HeaderAccept, ","); t != "" {
		this.metadata.Set(binder.HeaderAccept, t)
	}
	return this.metadata
}

func (this *httpProxy) ContentType(name string, split string) string {
	t := this.Context.Request.Header.Get(name)
	if t == "" {
		return ""
	}
	arr := strings.Split(t, split)
	for _, s := range arr {
		if b := binder.Get(s); b != nil {
			return b.Name()
		}
	}
	return ""
}
