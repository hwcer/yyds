package gateway

import (
	"errors"
	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosnet"
	"github.com/hwcer/cosrpc/xclient"
	"github.com/hwcer/cosrpc/xserver"
	"github.com/hwcer/coswss"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
	"github.com/soheilhy/cmux"
	"net"
	"strings"
	"time"
)

var mod = &Module{}

func New() *Module {
	return mod
}

type Module struct {
	mux    cmux.CMux
	Socket *Socket
	Server *Server
	//WebSocket *coswss.Server
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
	if i := strings.Index(options.Gate.Address, ":"); i < 0 {
		return errors.New("网关地址配置错误,格式: ip:port")
	} else if options.Gate.Address[0:i] == "" {
		options.Gate.Address = "0.0.0.0" + options.Gate.Address
	}
	if err = xserver.Start(); err != nil {
		return err
	}
	if err = xclient.Start(); err != nil {
		return err
	}

	return nil
}

func (this *Module) Start() (err error) {
	players.Start()
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
		if this.Socket == nil {
			this.Socket = &Socket{}
		}
		if err = this.Socket.init(); err != nil {
			return err
		}
		if this.mux != nil {
			so := this.mux.Match(cosnet.Matcher)
			err = this.Socket.Accept(so)
		} else {
			err = this.Socket.Listen(options.Gate.Address)
		}
		if err != nil {
			return err
		}
	}
	//http
	if p.Has(options.ProtocolTypeHTTP) {
		this.Server = &Server{}
		if err = this.Server.init(); err != nil {
			return err
		}
		if this.mux != nil {
			so := this.mux.Match(cmux.HTTP1Fast())
			err = this.Server.Accept(so)
		} else {
			err = this.Server.Listen(options.Gate.Address)
		}
		if err != nil {
			return err
		}
	}

	// websocket
	if p.Has(options.ProtocolTypeWSS) {
		if this.Socket == nil {
			this.Socket = &Socket{}
		}
		if err = this.Socket.init(); err != nil {
			return err
		}
		//this.WebSocket.Verify = WSVerify
		//this.WebSocket.Accept = WSAccept
		if p.Has(options.ProtocolTypeHTTP) {
			err = coswss.Binding(this.Server.Server, options.Options.Gate.Websocket)
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
func (this *Module) Reload() error {
	return nil
}
func (this *Module) Close() (err error) {
	if this.mux != nil {
		this.mux.Close()
	}
	return utils.Assert(xclient.Close, xserver.Close)

}