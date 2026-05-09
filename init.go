package yyds

import (
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/config"
)

func init() {
	logger.SetCallDepth(4)
	updater.Config.IMax = config.GetIMax
	updater.Config.IType = config.GetIType
	updater.Config.ParseId = ParseId
}

func ParseId(u *updater.Updater, oid string) (int32, error) {
	if i, _, err := uuid.Split(oid, uuid.BaseSize, 1); err != nil {
		return 0, err
	} else {
		return int32(i), nil
	}
}
