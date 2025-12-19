package handle

import (
	"github.com/hwcer/cosrpc/server"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/locator/model"
	"github.com/hwcer/yyds/options"
)

var db = model.DB()
var Service = server.Service(options.ServiceTypeLocator)

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
