package context

import (
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/registry"
	"strings"
)

const (
	ServiceMethodDebug       = "/debug"
	ServiceMethodRoleRenewal = "/role/renewal" //续约
)

var Verify func(*Context) (Token, error) //登录验证

//func Start() error {
//	//return loadAlphaAccount()
//	return nil
//}

func MethodGrade(serviceMethod string) int8 {
	if options.Gate.Prefix != "" {
		routePrefix := registry.Join(options.Gate.Prefix)
		serviceMethod = strings.TrimPrefix(serviceMethod, routePrefix)
	}
	return options.Apis.Get(serviceMethod)
}
