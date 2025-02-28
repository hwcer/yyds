package player

import "github.com/hwcer/cosgo/logger"

// Fields 角色字段名，一般情况下不需要设置
var Fields = &struct {
	Guid   string `json:"guid"`
	Create string `json:"create"`
}{
	Guid:   "Guid",
	Create: "Create",
}

const ProcessName = "_sys_process_player"

type itemGroup interface {
	GetId() int32
	GetNum() int32
}

type itemProbability interface {
	itemGroup
	GetVal() int32
}

type Emitter struct {
	Event   int32
	Daily   int32
	Record  int32
	Replace int32 //是否替换模式
}

// GetEmitter 获取事件配置
var GetEmitter = func(id int32) *Emitter {
	logger.Alert("请设置 player.GetEmitterConfig 才能使用全局事件")
	return nil
}
