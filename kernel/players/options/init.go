package options

import (
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/kernel/config"
	"github.com/hwcer/yyds/kernel/model"
)

func init() {
	updater.Config.IMax = config.GetIMax
	updater.Config.IType = config.GetIType
	updater.Config.ParseId = ParseId
}

func ParseId(u *updater.Updater, oid string) (int32, error) {
	if i, _, err := uuid.Split(oid, model.BaseSize, 1); err != nil {
		return 0, err
	} else {
		return int32(i), nil
	}
}
