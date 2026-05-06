package yyds

import (
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/options"
)

func init() {
	logger.SetCallDepth(4)
	updater.Config.IMax = options.Setting.GetIMax
	updater.Config.IType = options.Setting.GetIType
	updater.Config.ParseId = ParseId
}

func ParseId(u *updater.Updater, oid string) (int32, error) {
	if i, _, err := uuid.Split(oid, uuid.BaseSize, 1); err != nil {
		return 0, err
	} else {
		return int32(i), nil
	}
}
