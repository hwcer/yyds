package player

type itemGroup interface {
	GetId() int32
	GetNum() int32
}

type itemProbability interface {
	itemGroup
	GetVal() int32
}
