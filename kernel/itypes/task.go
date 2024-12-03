package itype

import (
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/kernel/config"
	"github.com/hwcer/yyds/kernel/model"
)

var Task = NewIType(config.ITypeTask)

func init() {
	Task.SetCreator(taskCreator)
	//if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeMaybe, &model.Task{}, Task); err != nil {
	//	logger.Panic(err)
	//}
}

func taskCreator(u *updater.Updater, iid int32, val int64) (any, error) {
	i := &model.Task{}
	i.Init(u, iid)
	i.OID, _ = Shop.ObjectId(u, iid)
	i.Value = int32(val)
	return i, nil
}
