package gateway

import (
	"strings"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/yyds/options"
)

func init() {
	cosgo.On(cosgo.EventTypStarted, func() error {
		if Options.OAuth == "" {
			return nil
		}
		servicePath, serviceMethod, err := Options.Router(Options.OAuth, nil)
		if err != nil {
			return err
		}
		options.OAuth.Set(servicePath, serviceMethod, options.OAuthTypeOAuth)
		return nil
	})
}

type router func(path string, req values.Metadata) (servicePath, serviceMethod string, err error)

var Options = struct {
	OAuth  string //业务服登录
	Router router //路由处理规则
}{
	OAuth:  "game/oauth",
	Router: defaultRouter,
}

// Router 默认路由处理方式
var defaultRouter router = func(path string, req values.Metadata) (servicePath, serviceMethod string, err error) {
	if strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}
	i := strings.Index(path, "/")
	if i < 0 {
		err = values.Errorf(404, "page not found")
		return
	}
	servicePath = strings.ToLower(path[0:i])
	serviceMethod = registry.Formatter(path[i:])
	return
}
