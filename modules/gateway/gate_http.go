package gateway

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosweb"
	"github.com/hwcer/cosweb/middleware"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/modules/gateway/players"
	"github.com/hwcer/yyds/options"
)

var ElapsedMillisecond = 200 * time.Millisecond

var Method = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
var Headers = []string{
	session.Options.Name,
	"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization",
	"X-CSRF-Token", "X-Requested-With", "X-Unity-Version", "x-Forwarded-Key", "x-Forwarded-Val",
}

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
	allow := middleware.NewAccessControlAllow()
	allow.Origin("*")
	allow.Methods(Method...)
	allow.Headers(strings.Join(Headers, ","))
	this.Server.Use(allow.Handle)
	this.Server.Register(Options.C2SOAuth, this.oauth)
	this.Server.Register("*", this.proxy, http.MethodPost)
	service := this.Server.Service()
	h := service.Handler().(*cosweb.Handler)
	h.SetSerialize(func(c *cosweb.Context, reply interface{}) ([]byte, error) {
		return Options.Serialize(c, reply)
	})

	if options.Gate.Static != nil && options.Gate.Static.Root != "" {
		static := this.Server.Static(options.Gate.Static.Route, options.Gate.Static.Root, http.MethodGet)
		if options.Gate.Static.Index != "" {
			static.Index(options.Gate.Static.Index)
		}
	}
	return nil
}

func (this *HttpServer) Listen(address string) (err error) {
	if options.Gate.KeyFile != "" && options.Gate.CertFile != "" {
		err = this.Server.TLS(address, options.Gate.CertFile, options.Gate.KeyFile)
	} else {
		err = this.Server.Listen(address)
	}
	if err == nil {
		logger.Trace("网关短连接启动：%v", options.Gate.Address)
	}
	return
}
func (this *HttpServer) Accept(ln net.Listener) (err error) {
	if options.Gate.KeyFile != "" && options.Gate.CertFile != "" {
		err = this.Server.TLS(ln, options.Gate.CertFile, options.Gate.KeyFile)
	} else {
		err = this.Server.Accept(ln)
	}
	if err == nil {
		logger.Trace("网关短连接启动：%v", options.Gate.Address)
	}
	return
}
func (this *HttpServer) oauth(c *cosweb.Context) any {
	authorize := &Authorize{}
	if err := c.Bind(&authorize); err != nil {
		return err
	}
	token, err := authorize.Verify()
	if err != nil {
		return err
	}
	h := httpProxy{Context: c}
	vs := values.Values{}
	if token.Developer {
		vs.Set(options.ServiceMetadataDeveloper, "1")
	} else {
		vs.Set(options.ServiceMetadataDeveloper, "")
	}
	if _, err = h.Login(token.Guid, vs); err != nil {
		return err
	}

	var v any
	v, err = oauth(&h)
	if err != nil {
		return err
	}
	return v
}

func (this *HttpServer) proxy(c *cosweb.Context) (r any) {
	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > ElapsedMillisecond {
			buff, _ := c.Buffer()
			logger.Alert("发现高延时请求,TIME:%v,PATH:%v,BODY:%v", elapsed, c.Request.URL.Path, string(buff.Bytes()))
		}
	}()
	h := &httpProxy{Context: c}
	p, err := h.Path()
	if err != nil {
		return err
	}
	var reply []byte
	if reply, err = caller(h, p); err != nil {
		return err
	}
	return reply
}

type httpProxy struct {
	*cosweb.Context
	uri      *url.URL
	cookie   *http.Cookie
	metadata values.Metadata
}

func (this *httpProxy) Path() (string, error) {
	return this.Context.Request.URL.Path, nil
}

func (this *httpProxy) Login(guid string, value values.Values) (token string, err error) {
	var data *session.Data
	token, data, err = players.Login(guid, value)
	if err != nil {
		return
	}
	//长连接顶号
	players.Replace(data, nil, this.Context.RemoteAddr())

	cookie := &http.Cookie{Name: session.Options.Name, Path: "/", Value: token}
	http.SetCookie(this.Context.Response, cookie)
	header := this.Header()
	header.Set("X-Forwarded-Key", session.Options.Name)
	header.Set("X-Forwarded-Val", cookie.Value)
	//this.Context.Set(session.Options.Name, cookie.Value)
	this.cookie = cookie

	return
}

func (this *httpProxy) Logout() error {
	return this.Context.Session.Delete()
}

func (this *httpProxy) Cookie() (*session.Data, error) {
	var token string
	if this.cookie != nil {
		token = this.cookie.Value
	} else {
		token = this.Context.GetString(session.Options.Name, cosweb.RequestDataTypeCookie, cosweb.RequestDataTypeQuery, cosweb.RequestDataTypeHeader)
	}
	if token == "" {
		return nil, values.Error("token empty")
	}
	if err := this.Context.Session.Verify(token); err != nil {
		return nil, err
	}
	return this.Context.Session.Data, nil
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
	if t := this.getContentType(binder.HeaderContentType, ";"); t != "" {
		this.metadata.Set(binder.HeaderContentType, t)
	} else {
		this.metadata.Set(binder.HeaderContentType, options.Options.Binder)
	}
	if t := this.getContentType(binder.HeaderAccept, ","); t != "" {
		this.metadata.Set(binder.HeaderAccept, t)
	}
	if this.cookie != nil {
		cookie := map[string]string{"name": session.Options.Name, "value": this.cookie.Value}
		b, _ := json.Marshal(cookie)
		this.metadata.Set(options.ServiceMetadataCookie, string(b))
	}
	return this.metadata
}
func (this *httpProxy) RemoteAddr() string {
	ip := this.Context.RemoteAddr()
	if i := strings.Index(ip, ":"); i > 0 {
		ip = ip[0:i]
	}
	return ip
}
func (this *httpProxy) getContentType(name string, split string) string {
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
