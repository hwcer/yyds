package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/gateway/rooms"
	"github.com/hwcer/yyds/options"
	"strings"
)

var cookiesAllowableName = map[string]struct{}{}

func SetCookieName(k string) {
	cookiesAllowableName[k] = struct{}{}
}

func init() {
	SetCookieName(options.ServiceMetadataUID)
	SetCookieName(options.ServiceMetadataServerId)
}

func CookiesFilter(cookie xshare.Metadata) values.Values {
	r := values.Values{}
	for k, v := range cookie {
		if _, ok := cookiesAllowableName[k]; ok {
			r[k] = v
		}
	}
	return r
}
func CookiesUpdate(cookie xshare.Metadata, p *session.Data) {
	vs := values.Values{}
	for k, v := range cookie {
		if strings.HasPrefix(k, options.ServicePlayerRoomJoin) {
			rooms.Join(v, p)
		} else if strings.HasPrefix(k, options.ServicePlayerRoomLeave) {
			rooms.Leave(v, p)
		} else if strings.HasPrefix(k, options.ServicePlayerSelector) {
			vs[k] = v
		} else if _, ok := cookiesAllowableName[k]; ok {
			vs[k] = v
		}
	}
	if len(vs) > 0 {
		p.Update(vs)
	}
}
