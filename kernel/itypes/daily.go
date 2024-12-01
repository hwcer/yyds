package itypes

import (
	"github.com/hwcer/yyds/kernel/config"
)

var Daily = NewIType(config.ITypeDaily)

//func init() {
//	im := &model.Daily{}
//	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, im, Daily); err != nil {
//		logger.Panic(err)
//	}
//}
