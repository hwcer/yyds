package itypes

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/game/model"
	"github.com/hwcer/yyds/game/share"
)

var Record = NewIType(share.ITypeRecord)

func init() {
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Record{}, Record); err != nil {
		logger.Panic(err)
	}
}
