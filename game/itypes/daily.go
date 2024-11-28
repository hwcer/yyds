package itypes

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/game/config"
	"github.com/hwcer/yyds/game/model"
)

var Daily = NewIType(config.ITypeDaily)

func init() {
	im := &model.Daily{}
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, im, Daily); err != nil {
		logger.Panic(err)
	}
}
