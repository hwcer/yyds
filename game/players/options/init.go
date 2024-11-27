package options

import (
	"github.com/hwcer/updater"
	"github.com/hwcer/uuid"
	"server/config"
	"server/game/model"
)

func init() {
	updater.Config.IMax = config.Data.GetIMax
	updater.Config.IType = config.Data.GetIType
	updater.Config.ParseId = ParseId
}

func ParseId(u *updater.Updater, oid string) (int32, error) {
	if i, _, err := uuid.Split(oid, model.BaseSize, 1); err != nil {
		return 0, err
	} else {
		return int32(i), nil
	}
}
