package itypes

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/kernel/config"
	"github.com/hwcer/yyds/kernel/model"
)

var Goods = NewIType(config.ITypeGoods)

func init() {
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Goods{}, Goods); err != nil {
		logger.Panic(err)
	}
}
