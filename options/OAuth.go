package options

import (
	"path"
	"strings"
)

// 接口权限设置

const (
	OAuthTypeNone    int8 = iota //不需要登录
	OAuthTypeOAuth               //需要认证
	OAuthTypeSelect              //需要选择角色,默认
	OAuthTypeRenewal             //需要续约时，仍然可以访问
)

var OAuth = authorizes{}

type authorizes map[string]int8

func init() {
	s := map[string]int8{
		"/ping":         OAuthTypeNone,
		"/login":        OAuthTypeNone,
		"/roles":        OAuthTypeOAuth,
		"/create":       OAuthTypeOAuth,
		"/select":       OAuthTypeOAuth,
		"/version":      OAuthTypeOAuth,
		"/reconnect":    OAuthTypeNone,
		"/role/create":  OAuthTypeOAuth,
		"/role/select":  OAuthTypeOAuth,
		"/role/renewal": OAuthTypeRenewal,
	}
	for k, v := range s {
		OAuth.Set(ServiceTypeGame, k, v)
	}
}

func (auth authorizes) Set(servicePath, serviceMethod string, i int8) {
	r := path.Join(servicePath, serviceMethod)
	r = strings.ToLower(r)
	if !strings.HasPrefix(r, "/") {
		r = "/" + r
	}
	auth[r] = i
}

func (auth authorizes) Get(s string) int8 {
	s = strings.ToLower(s)
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	if v, ok := auth[s]; !ok {
		return OAuthTypeSelect
	} else {
		return v
	}
}

func (auth authorizes) Range(f func(s string, i int8)) {
	for k, v := range auth {
		f(k, v)
	}
}
