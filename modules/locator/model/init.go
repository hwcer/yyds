package model

import (
	"errors"
	"fmt"

	"github.com/hwcer/cosmo"
	"github.com/hwcer/yyds/options"
)

var db = cosmo.New()

var Options = struct {
	Address string `json:"address"` //管理地址
	Mongodb string `json:"mongodb"`
	//master 事件总线地址，订阅其配置/服务器变更事件，空=不订阅。
	//按协议选择实现：tcp://host:port（默认，可省略 scheme）、redis://host:port
	Pubsub string `json:"pubsub"`
}{}

func DB() *cosmo.DB {
	return db
}

func Start() (err error) {
	if Options.Mongodb == "" {
		return errors.New("mongodb option is required")
	}
	if err = db.Start(fmt.Sprintf("%v#%v", options.Options.Appid, options.ServiceTypeLocator), Options.Mongodb); err != nil {
		return
	}
	return
}
