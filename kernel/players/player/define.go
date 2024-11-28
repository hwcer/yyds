package player

const ProcessNamePlayer = "_sys_process_player"

type itemGroup interface {
	GetId() int32
	GetNum() int32
}

type itemProbability interface {
	itemGroup
	GetVal() int32
}
