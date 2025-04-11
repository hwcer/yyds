package handle

import (
	"github.com/hwcer/cosrpc/xserver"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/social/model"
)

var db = model.DB()
var Service = xserver.Service(options.ServiceTypeSocial)

func Register(i interface{}, prefix ...string) {
	var arr []string
	if len(prefix) > 0 {
		arr = append(arr, prefix...)
	} else {
		arr = append(arr, "%v")
	}
	if err := Service.Register(i, arr...); err != nil {
		logger.Fatal("%v", err)
	}
}
