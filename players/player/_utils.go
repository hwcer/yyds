package player

import (
	"reflect"

	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/options"
)

func GetReqMeta(rp any) (req values.Metadata) {
	switch t := rp.(type) {
	case string:
		req = values.Metadata{}
		req.Set(options.ServiceMessagePath, t)
	case map[string]string:
		req = t
	case values.Metadata:
		req = t
	default:
		logger.Alert("unknown req type %v", reflect.TypeOf(rp))
		return
	}
	if _, ok := req[options.ServiceMessagePath]; !ok {
		logger.Alert("req no service message path:%v", rp)
		return
	}
	return
}
