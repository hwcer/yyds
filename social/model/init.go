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
}{}

func DB() *cosmo.DB {
	return db
}

func Start() (err error) {
	if Options.Mongodb == "" {
		return errors.New("mongodb option is required")
	}
	if err = db.Start(fmt.Sprintf("%v#%v", options.Options.Appid, options.ServiceTypeSocial), Options.Mongodb); err != nil {
		return
	}
	return
}
