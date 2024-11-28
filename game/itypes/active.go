package itypes

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/game/config"
	"github.com/hwcer/yyds/game/model"
)

var Active = NewIType(config.ITypeActive)
var Config = NewIType(config.ITypeConfig) //后台配置的活动

func init() {
	im := &model.Active{}
	Active.SetCreator(activeCreator)
	Config.SetCreator(activeCreator)

	var its []updater.IType
	its = append(its, Active, Config)
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeAlways, im, its...); err != nil {
		logger.Panic(err)
	}
}

func activeCreator(u *updater.Updater, iid int32, val int64) (any, error) {
	var err error
	i := &model.Active{}
	i.Model.Init(u, iid)
	i.OID, err = Active.ObjectId(u, iid)
	i.Update = u.Time.Unix()
	return i, err
}
