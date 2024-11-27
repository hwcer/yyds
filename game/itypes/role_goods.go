package itypes

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/game/model"
	"github.com/hwcer/yyds/game/share"
)

var Goods = NewIType(share.ITypeGoods)

func init() {
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Goods{}, Goods); err != nil {
		logger.Panic(err)
	}
}
