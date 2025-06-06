package player

var ITypeRole int32 = 10

// Fields 角色字段名，一般情况下不需要设置
var Fields = &struct {
	Guid   string `json:"guid"`
	Create string `json:"create"`
	Update string `json:"update"`
}{
	Guid:   "guid",
	Create: "create",
	Update: "update",
}

type itemGroup interface {
	GetId() int32
	GetNum() int32
}

type itemProbability interface {
	itemGroup
	GetVal() int32
}
