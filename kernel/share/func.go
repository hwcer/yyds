package share

import (
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosrpc/xshare"
	"strings"
)

func SetServiceAddress(c *xshare.Context, k, address string) {
	c.SetMetadata(options.GetServiceSelectorAddress(k), address)
}

// GetServiceMethod 获取外网使用的Method
func GetServiceMethod(method string) string {
	if options.Gate.Prefix == "" {
		return method
	}
	return registry.Join(options.Gate.Prefix, method)
}

// HasServiceMethod 判断是外网接口
func HasServiceMethod(path string) bool {
	if options.Gate.Prefix == "" {
		return true //无法判断
	}
	path = strings.TrimPrefix(path, "/")
	return strings.HasPrefix(path, options.Gate.Prefix)
}
