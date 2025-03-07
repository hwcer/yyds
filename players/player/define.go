package player

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
