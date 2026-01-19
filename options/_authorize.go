package options

import (
	"github.com/hwcer/gateway/gwcfg"
)

//// 接口权限设置
//
//type OAuthType int8
//
//const (
//	OAuthTypeNone   OAuthType = iota //不需要登录
//	OAuthTypeOAuth                   //需要认证
//	OAuthTypeSelect                  //需要选择角色,但不需要进入用户协程，无法直接操作用户数据
//	OAuthTypePlayer                  // 需要选择角色,并进入用户协程 默认
//)
//
//var OAuthRenewal = "/game/role/renewal"
//
//var OAuth = authorizes{dict: map[string]OAuthType{}, prefix: map[string]OAuthType{}, v: OAuthTypePlayer}
//
//type authorizes struct {
//	v      OAuthType //默认
//	dict   map[string]OAuthType
//	prefix map[string]OAuthType //按前缀匹配
//	master map[string]struct{}  //是否master
//}

func init() {
	s := map[string]gwcfg.OAuthType{
		"/ping":        gwcfg.OAuthTypeNone,
		"/login":       gwcfg.OAuthTypeNone,
		"/roles":       gwcfg.OAuthTypeOAuth,
		"/create":      gwcfg.OAuthTypeOAuth,
		"/select":      gwcfg.OAuthTypeOAuth,
		"/version":     gwcfg.OAuthTypeOAuth,
		"/reconnect":   gwcfg.OAuthTypeNone,
		"/role/create": gwcfg.OAuthTypeOAuth,
		"/role/select": gwcfg.OAuthTypeOAuth,
	}
	for k, v := range s {
		gwcfg.Authorize.Set(ServiceTypeGame, k, v)
	}
}

//
//func (auth *authorizes) Format(s ...string) string {
//	var r string
//	if len(s) > 1 {
//		r = path.Join(s...)
//	} else if len(s) == 1 {
//		r = s[0]
//	} else {
//		return ""
//	}
//
//	r = strings.ToLower(r)
//	if !strings.HasPrefix(r, "/") {
//		r = "/" + r
//	}
//	return r
//}
//
//func (auth *authorizes) Set(servicePath, serviceMethod string, i OAuthType) {
//	r := auth.Format(servicePath, serviceMethod)
//	auth.dict[r] = i
//}
//
//func (auth *authorizes) Get(s ...string) (v OAuthType, path string) {
//	path = auth.Format(s...)
//	var ok bool
//	if v, ok = auth.dict[path]; ok {
//		return
//	}
//	var k string
//	for k, v = range auth.prefix {
//		if strings.HasPrefix(path, k) {
//			return
//		}
//	}
//	v = auth.v
//	return
//}
//
//func (auth *authorizes) Prefix(servicePath, serviceMethod string, i OAuthType) {
//	r := auth.Format(servicePath, serviceMethod)
//	auth.prefix[r] = i
//}
//
//// Default 设置,获取默认值
//func (auth *authorizes) Default(l ...OAuthType) OAuthType {
//	if len(l) > 0 {
//		auth.v = l[0]
//	}
//	return auth.v
//}
//
//// SetMaster 前缀模式匹配
//func (auth *authorizes) SetMaster(servicePath string, serviceMethod string) {
//	if auth.master == nil {
//		auth.master = map[string]struct{}{}
//	}
//	r := auth.Format(servicePath, serviceMethod)
//	auth.master[r] = struct{}{}
//}
//
//func (auth *authorizes) IsMaster(path string) bool {
//	for p, _ := range auth.master {
//		if strings.HasPrefix(path, p) {
//			return true
//		}
//	}
//	return false
//}
