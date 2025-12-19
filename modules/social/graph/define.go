package graph

import "github.com/hwcer/cosgo/values"

type Relation uint8

const (
	RelationNone     Relation = 0      //无关系，但可能有互动
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
	Create(uid string) (values.Values, error)
	SendMessage(uid string, path string, data any)
}

type Getter struct {
	sg  *Graph
	fid string
	fri *Friend
}

func (this *Getter) Fid() string {
	return this.fid
}

func (this *Getter) Player() *Player {
	return this.sg.nodes[this.fid]
}
func (this *Getter) Friend() *Friend {
	return this.fri
}

// Statement 通过Lock RLock 获得的临时无锁操作
type Statement struct {
	g *Graph
}

func (this *Statement) Get(uid string) *Player {
	return this.g.nodes[uid]
}
