package gateway

import (
	"bytes"
	"fmt"

	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
)

type Request interface {
	Path() (string, error)
	Login(guid string, value values.Values) error //登录
	Logout() error                                //退出登录
	Cookie() (*session.Data, error)               //当前登录信息
	Buffer() (buf *bytes.Buffer, err error)
	Metadata() values.Metadata
	RemoteAddr() string
}

func oauth(h Request) (any, error) {
	if Options.Verify == "" {
		return true, nil
	}
	return caller(h, Options.Verify)
}

func caller(h Request, path string) ([]byte, error) {
	req := h.Metadata()
	res := make(values.Metadata)
	var p *session.Data
	servicePath, serviceMethod, err := Options.Router(path, req)
	if err != nil {
		return nil, err
	}

	l, s := options.OAuth.Get(servicePath, serviceMethod)
	isMaster := options.OAuth.IsMaster(s)
	if f, ok := Access.dict[l]; !ok {
		return nil, fmt.Errorf("unknown authorization type: %d", l)
	} else if p, err = f(h, req, isMaster); err != nil {
		return nil, err
	}

	req.Set(options.ServicePlayerGateway, cosrpc.Address().Encode())
	//使用用户级别微服务筛选器
	if p != nil {
		if serviceAddress := p.GetString(options.GetServiceSelectorAddress(servicePath)); serviceAddress != "" {
			req.Set(options.SelectorAddress, serviceAddress)
		}
	}
	var buff *bytes.Buffer
	if buff, err = h.Buffer(); err != nil {
		return nil, err
	}
	reply := make([]byte, 0)
	
	if Options.Request != nil {
		Options.Request(p, s, req)
	}

	if options.Gate.Prefix != "" {
		serviceMethod = registry.Join(options.Gate.Prefix, serviceMethod)
	}

	if err = client.CallWithMetadata(req, res, servicePath, serviceMethod, buff.Bytes(), &reply); err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return reply, nil
	}
	//创建登录信息
	if guid, ok := res[options.ServicePlayerOAuth]; ok {
		if err = h.Login(guid, CookiesFilter(res)); err != nil {
			return nil, err
		} else {
			p = players.Get(guid)
		}
	}
	//退出登录
	if _, ok := res[options.ServicePlayerLogout]; ok {
		if err = h.Logout(); err == nil && p != nil {
			players.Delete(p)
		}
		p = nil
	}

	if p != nil {
		CookiesUpdate(res, p)
	}
	if err != nil {
		return nil, err
	} else {
		return reply, nil
	}
}
