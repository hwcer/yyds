package model

import (
	"github.com/hwcer/cosgo/logger"
	"strings"
)

var Handle = roleValuesHandles{}

type roleInit interface {
	Init(*Role)
}

type roleValuesHandle interface {
	getter(r *Role, k string) (any, bool)        //get子级数据
	setter(r *Role, k string, v any) (any, bool) //set子级数据
}

type roleValuesHandles map[string]roleValuesHandle

func (h roleValuesHandles) Get(name string) roleValuesHandle {
	name = strings.ToLower(name)
	return h[name]
}

func (h roleValuesHandles) Register(name string, handle roleValuesHandle) {
	name = strings.ToLower(name)
	if _, ok := h[name]; ok {
		logger.Fatal("role handle[%v]exist", name)
	}
	h[name] = handle
}

func (r *Role) getFromHandle(k string) (v any, ok bool) {
	if i := strings.Index(k, "."); i > 0 {
		s := strings.ToLower(k[0:i])
		handle := Handle.Get(s)
		if handle == nil {
			logger.Alert("role handle[%v] not found", k)
			return nil, true
		}
		return handle.getter(r, k[i+1:])
	}
	return nil, false
}
func (r *Role) setFromHandle(k string, v any) (any, bool) {
	if i := strings.Index(k, "."); i > 0 {
		s := strings.ToLower(k[0:i])
		handle := Handle.Get(s)
		if handle == nil {
			logger.Alert("role handle[%v] not found", k)
			return v, true
		}
		return handle.setter(r, k[i+1:], v)
	}
	return v, false
}
