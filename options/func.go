package options

import (
	"github.com/hwcer/cosgo/registry"
	"strings"
)

// GetServiceMethod 获取外网使用的Method
func GetServiceMethod(method string) string {
	if Gate.Prefix == "" {
		return method
	}
	return registry.Join(Gate.Prefix, method)
}

// HasServiceMethod 判断是外网接口
func HasServiceMethod(path string) bool {
	if Gate.Prefix == "" {
		return true //无法判断
	}
	path = strings.TrimPrefix(path, "/")
	return strings.HasPrefix(path, Gate.Prefix)
}

func TrimServiceMethod(path string) string {
	if Gate.Prefix == "" {
		return path
	}
	path = strings.TrimPrefix(path, "/")
	return strings.TrimPrefix(path, Gate.Prefix)
}

func GetServerTime() int64 {
	return Game.timeUnix
}
