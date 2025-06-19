package options

import (
	"path"
	"strings"
)

// 接口权限设置

const (
	OAuthTypeNone   int8 = iota //不需要登录
	OAuthTypeOAuth              //需要认证
	OAuthTypeSelect             //需要选择角色,默认
	OAuthTypeMaster             //需要GM权限
)

var OAuthRenewal = "/game/role/renewal"

var OAuth = authorizes{dict: map[string]int8{}, prefix: map[string]int8{}}

type authorizes struct {
	dict   map[string]int8
	prefix map[string]int8 //按前缀匹配
}

func init() {
	s := map[string]int8{
		"/ping":        OAuthTypeNone,
		"/login":       OAuthTypeNone,
		"/roles":       OAuthTypeOAuth,
		"/create":      OAuthTypeOAuth,
		"/select":      OAuthTypeOAuth,
		"/version":     OAuthTypeOAuth,
		"/reconnect":   OAuthTypeNone,
		"/role/create": OAuthTypeOAuth,
		"/role/select": OAuthTypeOAuth,
	}
	for k, v := range s {
		OAuth.Set(ServiceTypeGame, k, v)
	}
}
func (auth *authorizes) Format(s ...string) string {
	var r string
	if len(s) > 1 {
		r = path.Join(s...)
	} else if len(s) == 1 {
		r = s[0]
	} else {
		return ""
	}

	r = strings.ToLower(r)
	if !strings.HasPrefix(r, "/") {
		r = "/" + r
	}
	return r
}

func (auth *authorizes) Set(servicePath, serviceMethod string, i int8) {
	r := auth.Format(servicePath, serviceMethod)
	auth.dict[r] = i
}

func (auth *authorizes) Get(s ...string) (int8, string) {
	p := auth.Format(s...)
	if v, ok := auth.dict[p]; ok {
		return v, p
	}
	for k, v := range auth.prefix {
		if strings.HasPrefix(p, k) {
			return v, p
		}
	}

	return OAuthTypeSelect, p
}

func (auth *authorizes) Prefix(servicePath, serviceMethod string, i int8) {
	r := auth.Format(servicePath, serviceMethod)
	auth.prefix[r] = i
}
