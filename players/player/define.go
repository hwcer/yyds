package player

import (
	"github.com/hwcer/cosgo/logger"
)

const ProcessName = "_sys_process_player"

type itemGroup interface {
	GetId() int32
	GetNum() int32
}

type itemProbability interface {
	itemGroup
	GetVal() int32
}

type EmitterConfig interface {
	GetDaily() int32
	GetRecord() int32
	GetEvents() int32
	GetUpdate() int32
	GetReplace() int32
}

// GetRoleCreateTime 角色创建时间
var GetRoleCreateTime = func(player *Player) int64 {
	logger.Alert("请设置 player.GetRoleCreateTime 才能正确使用角色创建时间")
	return 0
}

// GetEmitterConfig 获取事件配置
var GetEmitterConfig = func(id int32) EmitterConfig {
	logger.Alert("请设置 player.GetEmitterConfig 才能使用全局事件")
	return nil
}
