package gateway

import (
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
	"strings"
)

type router func(path string, req values.Metadata) (servicePath, serviceMethod string, err error)
type errorf func(*cosnet.Context, error) any

// Errorf 默认错误处理方式
var Errorf errorf = func(c *cosnet.Context, err error) any {
	return values.Error(err)
}

// Router 默认路由处理方式
var Router router = func(path string, req values.Metadata) (servicePath, serviceMethod string, err error) {
	i := strings.Index(path, "/")
	if i < 0 {
		err = values.Errorf(404, "page not found")
		return
	}
	servicePath = strings.ToLower(path[0:i])
	serviceMethod = registry.Formatter(path[i:])
	return
}
