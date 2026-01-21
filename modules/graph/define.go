package graph

import "github.com/hwcer/cosgo/values"

type Relation uint8

const (
	RelationNone     Relation = 0      //无关系(任意关系)，但可能有互动
	RelationFans     Relation = 1      //我的粉丝
	RelationFollow   Relation = 1 << 1 //我的关注
	RelationFriend   Relation = 1 << 2 //我的好友
	RelationUnfriend Relation = 1 << 7 //黑名单

)

func (r Relation) Has(v Relation) bool {
	return r&v == v
}

func (r Relation) Set(v Relation) Relation {
	if r&v == v {
		return r
	}
	return r | v
}

type Factory interface {
	Limit(uid string) int32                                //好友上限
	Create(uid string) (values.Values, error)              //创建用户图谱时
	SetPlayer(uid string, key string, val any)             //修改 player.Values 中的信息时回调
	SetFriend(uid string, fid string, key string, val any) //修改 friend.Values (我的好友) 中的信息时回调
	SendMessage(uid string, path string, data any)         //发送消息
}

// Statement 通过Lock RLock 获得的临时无锁操作
//type Statement struct {
//	g *Graph
//}
//
//func (this *Statement) Get(uid string) *Player {
//	return this.g.nodes[uid]
//}
