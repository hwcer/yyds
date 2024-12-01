package kernel

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/kernel/itypes"
	"github.com/hwcer/yyds/kernel/model"
)

func init() {

	its := []updater.IType{itypes.Role, itypes.ItemsGroup, itypes.ItemsPacks}
	//ROLE
	if err := updater.Register(updater.ParserTypeDocument, updater.RAMTypeAlways, &model.Role{}, its...); err != nil {
		logger.Panic(err)
	}
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Goods{}, itypes.Goods); err != nil {
		logger.Panic(err)
	}
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Record{}, itypes.Record); err != nil {
		logger.Panic(err)
	}
	//Active
	its = []updater.IType{itypes.Active, itypes.Config}
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeAlways, &model.Active{}, its...); err != nil {
		logger.Panic(err)
	}

	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Daily{}, itypes.Daily); err != nil {
		logger.Panic(err)
	}

	its = []updater.IType{itypes.Items, itypes.Viper, itypes.Gacha, itypes.Ticket}
	its = append(its, itypes.Equip, itypes.Hero)
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeAlways, &model.Items{}, its...); err != nil {
		logger.Panic(err)
	}
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeNone, &model.Shop{}, itypes.Shop); err != nil {
		logger.Panic(err)
	}
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeMaybe, &model.Task{}, itypes.Task); err != nil {
		logger.Panic(err)
	}
}
