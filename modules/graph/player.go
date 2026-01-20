package graph

import (
	"time"

	"github.com/hwcer/cosgo/values"
)

func NewPlayer(uid string, v values.Values) *Player {
	if v == nil {
		v = values.Values{}
	}
	return &Player{uid: uid, Values: v, friends: map[string]*Friend{}}
}

func NewFriend(r Relation, v values.Values) *Friend {
	if v == nil {
		v = values.Values{}
	}
	return &Friend{Values: values.Values{}, relation: r}
}

type Friend struct {
	values.Values          //存储的业务数据,我对好友干了什么事情
	relation      Relation //好友关系
}

func (f *Friend) Has(v Relation) bool {
	return f.relation.Has(v)
}

func (f *Friend) Relation() Relation {
	return f.relation
}

type Player struct {
	uid           string
	values.Values       //存储的业务数据，我今天干了什么
	Update        int64 //上次更新  Values 用于增量获取好友信息
	friends       map[string]*Friend
}

func (p *Player) Uid() string {
	return p.uid
}
func (p *Player) Set(key string, val any) {
	p.Values.Set(key, val)
	p.Update = time.Now().Unix()
}

func (p *Player) Has(fid string, relation ...Relation) bool {
	f := p.friends[fid]
	if f == nil {
		return false
	}
	for _, v := range relation {
		if f.Has(v) {
			return true
		}
	}
	return false
}

// IsMax 好友是否已满
func (p *Player) IsMax(limit int32) bool {
	if limit <= 0 {
		return false
	}
	if p.Count(RelationFriend) >= limit {
		return true
	}
	return false
}

func (p *Player) Count(t Relation) (r int32) {
	for _, friend := range p.friends {
		if friend.relation.Has(t) {
			r++
		}
	}
	return
}

func (p *Player) Relation(fid string) Relation {
	f := p.friends[fid]
	if f == nil {
		return RelationNone
	}
	return f.relation
}

// Friend 获取我的好友信息
func (p *Player) Friend(fid string) *Friend {
	return p.friends[fid]
}

// Modify 修改 或者 创建好友关系
// 删除时 relation =0 就行，不需要删除数据，避免交互信息丢失，造成可以反复删除、添加好友 刷礼物BUG
func (p *Player) Modify(fid string, r Relation) *Friend {
	v, ok := p.friends[fid]
	if !ok {
		v = NewFriend(r, nil)
		p.friends[fid] = v
	} else {
		v.relation = r
	}

	return v
}

func (p *Player) Remove(fid string) *Friend {
	v, ok := p.friends[fid]
	if ok {
		if v.relation == RelationUnfriend {
			return v
		}
		if len(v.Values) == 0 {
			delete(p.friends, fid)
		} else {
			v.relation = RelationNone
		}
	}
	return v
}

// Unfriend 拉黑
func (p *Player) Unfriend(fid string) {
	p.friends[fid] = NewFriend(RelationUnfriend, nil)
}
