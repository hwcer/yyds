package itypes

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/kernel/config"
	"github.com/hwcer/yyds/kernel/model"
)

var Record = NewIType(config.ITypeRecord)

func init() {
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Record{}, Record); err != nil {
		logger.Panic(err)
	}
}
