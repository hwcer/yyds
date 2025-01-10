package operator

func New(t Types, v int64, r any) *Operator {
	return &Operator{Type: t, Value: v, Result: r}
}

type Operator struct {
	OID    string `json:"o,omitempty"` //object id
	IID    int32  `json:"i,omitempty"` //item id
	Key    string `json:"k,omitempty"` //字段名
	Bag    int32  `json:"b,omitempty"` //物品类型 model
	Type   Types  `json:"t"`           //操作类型 opt
	Value  int64  `json:"v"`           //增量,add sub new 时有效
	Result any    `json:"r"`           //最终结果
}

func (opt *Operator) SetKey(k string) {
	opt.Key = k
}
func (opt *Operator) SetOID(id string) {
	opt.OID = id
}

/*
	数据结构以及有效字段说明

	公共字段，所有模式下都存在，且意义相同：Bag,Type

	ParserTypeValues :
		ADD : IID (int32),Value (int32),Result (int32)
		SUB : IID (int32),Value (int32),Result (int32)
		SET : IID (int32),Result (int32)
		DEL : IID (int32)


    ParserTypeDocument :
		ADD : Key(string),Value(any),Result(any)
		SUB : Key(string),Value(any),Result(any)
		SET : Key(string),Result(any) {b=10  t = set  k=lv r=10}

	ParserTypeCollection:
		ADD : OID(string),IID(int32),Value(int32),Result(int32)
		SUB : OID(string),IID(int32),Value(int32),Result(int32)
		DEL : OID(string),IID(int32)
		SET : OID(string),IID(int32),Result(map(string)any)
		NEW : OID(string),IID(int32),Result(any)
*/
