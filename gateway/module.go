package gateway

import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosnet"
	"github.com/hwcer/coswss"
	"github.com/hwcer/yyds/options"
	"github.com/soheilhy/cmux"
)

var mod = &Module{}

var TCP = NewTCPServer()
var HTTP = NewHttpServer()

func New() *Module {
	return mod
}

type Module struct {
	mux cmux.CMux
}

func (this *Module) Id() string {
	return options.ServiceTypeGate
}

func (this *Module) Init() (err error) {
	if err = options.Initialize(); err != nil {
		return
	}
	if options.Gate.Address == "" {
		return errors.New("网关地址没有配置")
	}
	session.Heartbeat.Start()
	//session
	if options.Gate.Redis != "" {
		session.Options.Storage, err = session.NewRedis(options.Gate.Redis)
	} else {
		session.Options.Storage = session.NewMemory(options.Gate.Capacity)
	}
	if err != nil {
		return err
	}

	if i := strings.Index(options.Gate.Address, ":"); i < 0 {
		return errors.New("网关地址配置错误,格式: ip:port")
	} else if options.Gate.Address[0:i] == "" {
		options.Gate.Address = "0.0.0.0" + options.Gate.Address
	}
	p := options.Gate.Protocol
	if p.Has(options.ProtocolTypeTCP) || p.Has(options.ProtocolTypeWSS) {
		if err = TCP.init(); err != nil {
			return err
		}
	}
	if p.Has(options.ProtocolTypeHTTP) {
		if err = HTTP.init(); err != nil {
			return err
		}
	}

	return nil
}

func (this *Module) Start() (err error) {
	if options.Gate.Protocol.CMux() {
		var ln net.Listener
		if ln, err = net.Listen("tcp", options.Gate.Address); err != nil {
			return err
		}
		this.mux = cmux.New(ln)
	}
	p := options.Gate.Protocol
	//SOCKET
	if p.Has(options.ProtocolTypeTCP) {
		if this.mux != nil {
			so := this.mux.Match(cosnet.Matcher)
			err = TCP.Accept(so)
		} else {
			err = TCP.Listen(options.Gate.Address)
		}
		if err != nil {
			return err
		}
	}
	//http
	if p.Has(options.ProtocolTypeHTTP) {
		if this.mux != nil {
			so := this.mux.Match(cmux.HTTP1Fast())
			err = HTTP.Accept(so)
		} else {
			err = HTTP.Listen(options.Gate.Address)
		}
		if err != nil {
			return err
		}
	}

	// websocket
	if p.Has(options.ProtocolTypeWSS) {
		if p.Has(options.ProtocolTypeHTTP) {
			err = coswss.Binding(HTTP.Server, options.Options.Gate.Websocket)
		} else {
			err = coswss.Listen(options.Gate.Address, options.Options.Gate.Websocket)
		}
		if err != nil {
			return err
		}
	}

	if this.mux != nil {
		err = scc.Timeout(time.Second, func() error { return this.mux.Serve() })
		if errors.Is(err, scc.ErrorTimeout) {
			err = nil
		}
	}

	return err
}

func (this *Module) Close() (err error) {
	if this.mux != nil {
		this.mux.Close()
	}
	return nil
}
