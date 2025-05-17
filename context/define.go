package context

import (
	"github.com/hwcer/yyds/options"
	"path"
)

const (
	ServiceMethodDebug = "/debug"
)

func MethodGrade(s string) (l int8, p string) {
	p = options.TrimServiceMethod(s)
	m := path.Join(options.ServiceTypeGame, p)
	l = options.OAuth.Get(m)
	return
}
