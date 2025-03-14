package gateway

import (
	"bytes"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/xclient"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
	"strings"
)

type Request interface {
	Path() (string, error)
	Data() (*session.Data, error)
	Login(guid string, cookie values.Values) error
	Buffer() (buf *bytes.Buffer, err error)
	Delete() error
	Metadata() values.Metadata
}

// request rpc转发,返回实际转发的servicePath
func request(p *session.Data, servicePath, serviceMethod string, args []byte, req, res values.Metadata, reply any) error {
	if options.Gate.Prefix != "" {
		serviceMethod = registry.Join(options.Gate.Prefix, serviceMethod)
	}
	if p != nil {
		if serviceAddress := p.GetString(options.GetServiceSelectorAddress(servicePath)); serviceAddress != "" {
			req.Set(options.SelectorAddress, serviceAddress)
		}
	}

	return xclient.CallWithMetadata(req, res, servicePath, serviceMethod, args, reply)
}

func proxy(h Request) ([]byte, error) {
	path, err := h.Path()
	if err != nil {
		return nil, err
	}
	req := h.Metadata()
	res := make(values.Metadata)
	var p *session.Data

	if strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}
	servicePath, serviceMethod, err := Router(path, req)
	if err != nil {
		return nil, err
	}
	path = strings.Join([]string{servicePath, strings.TrimPrefix(serviceMethod, "/")}, "/")

	limit := options.OAuth.Get(path)
	if limit != options.OAuthTypeNone {
		if p, err = h.Data(); err != nil {
			return nil, values.Parse(err)
		} else if p == nil {
			return nil, errors.ErrLogin
		}
		p.KeepAlive()
		if limit == options.OAuthTypeOAuth {
			if uuid := p.UUID(); uuid == "" {
				return nil, errors.ErrLogin
			} else {
				req[options.ServiceMetadataGUID] = p.UUID()
			}
		} else {
			if uid := p.GetString(options.ServiceMetadataUID); uid == "" {
				return nil, errors.ErrNotSelectRole
			} else {
				req[options.ServiceMetadataUID] = p.GetString(options.ServiceMetadataUID)
			}
		}
	}
	req.Set(options.ServicePlayerGateway, xshare.Address().Encode())
	buff, err := h.Buffer()
	if err != nil {
		return nil, err
	}
	reply := make([]byte, 0)
	Emitter.emit(EventTypeRequest, p, path, req)
	if err = request(p, servicePath, serviceMethod, buff.Bytes(), req, res, &reply); err != nil {
		return nil, err
	}
	Emitter.emit(EventTypeConfirm, p, path, req)
	if len(res) == 0 {
		return reply, nil
	}
	if guid, ok := res[options.ServicePlayerOAuth]; ok {
		err = h.Login(guid, CookiesFilter(res))
	} else if _, ok = res[options.ServicePlayerLogout]; ok {
		if err = h.Delete(); err == nil && p != nil {
			players.Delete(p)
		}
	} else if p != nil {
		CookiesUpdate(res, p)
	}
	if err != nil {
		return nil, err
	} else {
		return reply, nil
	}
}
