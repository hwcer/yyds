package graph

import "github.com/hwcer/cosgo/values"

// 程序启动时预加载使用，不需要进入锁状态
type Install struct {
	g *Graph
}

func (i *Install) GetPlayer(uid string) *Player {
	p, _ := i.g.load(uid)
	return p
}

func (i *Install) SetPlayer(uid string, value values.Values) {
	i.g.nodes[uid] = NewPlayer(value)
}
func (i *Install) SetFriend(uid string, fid string, relation Relation, val values.Values) (*Friend, error) {
	p, err := i.g.load(uid)
	if err != nil {
		return nil, err
	}
	if _, err = i.g.load(fid); err != nil {
		return nil, err
	}
	fri := NewFriend(relation, val)
	p.friends[fid] = fri
	return fri, nil
}
