package graph

import "github.com/hwcer/cosgo/values"

func NewFriend(r Relation, v values.Values) *Friend {
	if v == nil {
		v = values.Values{}
	}
	return &Friend{Values: v, relation: r}
}

type Friend struct {
	values.Values          //存储的业务数据,我对好友干了什么事情
	relation      Relation //好友关系
}

func (f *Friend) Has(v Relation) bool {
	if v == RelationNone {
		return true
	}
	return f.relation.Has(v)
}

func (f *Friend) Relation() Relation {
	return f.relation
}
