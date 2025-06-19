package context

import (
	"github.com/hwcer/yyds/options"
)

const (
	ServiceMethodDebug = "/debug"
)

func MethodGrade(s string) (l int8, p, m string) {
	p = options.TrimServiceMethod(s)
	l, m = options.OAuth.Get(options.ServiceTypeGame, p)
	return
}
