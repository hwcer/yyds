package gateway

import (
	"strings"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/yyds/gateway/channel"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
)

func init() {
	cosgo.On(cosgo.EventTypStarted, func() error {
		//设置登录权限
		if Options.Verify != "" {
			servicePath, serviceMethod, err := Options.Router(Options.Verify, nil)
			if err != nil {
				return err
			}
			options.OAuth.Set(servicePath, serviceMethod, options.OAuthTypeOAuth)
		}
		//监控登录信息
		session.OnRelease(func(data *session.Data) {
			_ = players.Delete(data)
			channel.Release(data)
		})
		return nil
	})
}

var Options = struct {
	OAuth     string //网关登录
	Verify    string //游戏服登录验证,留空不进行验证
	Router    router //路由处理规则
	Serialize func(c Context, reply any) ([]byte, error)
}{
	OAuth:     "oauth",
	Verify:    "game/oauth",
	Router:    defaultRouter,
	Serialize: defaultSerialize,
}

type router func(path string, req values.Metadata) (servicePath, serviceMethod string, err error)

// Router 默认路由处理方式
var defaultRouter router = func(path string, req values.Metadata) (servicePath, serviceMethod string, err error) {
	path = strings.TrimPrefix(path, "/")
	i := strings.Index(path, "/")
	if i < 0 {
		err = values.Errorf(404, "page not found")
		return
	}
	servicePath = registry.Formatter(path[0:i])
	serviceMethod = registry.Formatter(path[i:])
	return
}

type Context interface {
	Accept() binder.Binder
}

func defaultSerialize(c Context, reply any) ([]byte, error) {
	b := c.Accept()
	v := values.Parse(reply)
	return b.Marshal(v)
}
