package kernel

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/kernel/itype"
	"github.com/hwcer/yyds/kernel/model"
)

func init() {

	its := []updater.IType{itype.Role, itype.ItemsGroup, itype.ItemsPacks}
	//ROLE
	if err := updater.Register(updater.ParserTypeDocument, updater.RAMTypeAlways, &model.Role{}, its...); err != nil {
		logger.Panic(err)
	}
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Goods{}, itype.Goods); err != nil {
		logger.Panic(err)
	}
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Record{}, itype.Record); err != nil {
		logger.Panic(err)
	}
	//Active
	its = []updater.IType{itype.Active, itype.Config}
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeAlways, &model.Active{}, its...); err != nil {
		logger.Panic(err)
	}

	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, &model.Daily{}, itype.Daily); err != nil {
		logger.Panic(err)
	}

	its = []updater.IType{itype.Items, itype.Viper, itype.Gacha, itype.Ticket}
	its = append(its, itype.Equip, itype.Hero)
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeAlways, &model.Items{}, its...); err != nil {
		logger.Panic(err)
	}
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeNone, &model.Shop{}, itype.Shop); err != nil {
		logger.Panic(err)
	}
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeMaybe, &model.Task{}, itype.Task); err != nil {
		logger.Panic(err)
	}
}
