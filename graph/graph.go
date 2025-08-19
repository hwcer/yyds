package graph

import (
	"sync"
)

type Graph struct {
	mu      sync.RWMutex     // 读写锁保证并发安全
	nodes   map[string]*node // 用户ID到用户对象的映射
	factory Factory          // 用户工厂函数
}

// New 创建一个新的社交图谱
func New(userFactory Factory) (g *Graph, i Install) {
	g = &Graph{
		nodes:   make(map[string]*node),
		factory: userFactory,
	}
	i = Install{g: g}
	return
}

// 获取，或者创建
func (sg *Graph) load(uid string) (r *node, err error) {
	if r = sg.nodes[uid]; r == nil {
		var p Player
		if p, err = sg.factory.Player(uid); err != nil {
			return
		} else if p == nil {
			return nil, ErrorUserNotExist
		}
		r = newNode(p)
		sg.nodes[uid] = r
	}
	return
}

// Add 添加新用户
func (sg *Graph) Add(uid string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if _, exists := sg.nodes[uid]; !exists {
		if p, _ := sg.factory.Player(uid); p != nil {
			sg.nodes[uid] = newNode(p)
		}
	}
}
func (sg *Graph) Has(uid, tar string) bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	n := sg.nodes[uid]
	if n == nil {
		return false
	}
	_, ok := n.friends[tar]

	return ok
}

// Get 获取我的/好友数据
func (sg *Graph) Get(uid string, tar string) Friend {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	me := sg.nodes[uid]
	if me == nil {
		return nil
	}
	return me.friends[tar]
}

func (sg *Graph) Player(uid string) Player {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	me := sg.nodes[uid]
	if me == nil {
		return nil
	}
	return me.p
}

// Follow 关注好友，
// 如果对方也关注自己，则直接成为好友关系
// fri 直接成为好友
func (sg *Graph) Follow(uid, tar string) (fri bool, err error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	var p, t *node
	if p, err = sg.load(uid); err != nil {
		return
	}
	if t, err = sg.load(tar); err != nil {
		return
	}
	switch p.Has(tar) {
	case FriendshipNone:
		p.Follow(tar)
		t.Fans(uid)
	case FriendshipFans:
		fri = true
		p.Add(sg.factory.Friend(t.p))
		t.Add(sg.factory.Friend(p.p))
		//case FriendshipFriend, FriendshipFollow:
	}
	return
}

// Delete 移除好友关系
func (sg *Graph) Delete(uid, tar string) (r Friend) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	// 双向移除好友关系
	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	if r = p.Delete(tar); r == nil {
		return nil
	}

	t := sg.nodes[tar]
	if t == nil {
		return nil
	}
	t.Delete(uid)
	return
}

// Accept 接受好友申请
// 返回 成功加为好友的列表
func (sg *Graph) Accept(uid string, tar ...string) (success []string, err error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	var p *node
	if p, err = sg.load(uid); err != nil {
		return
	}
	if len(tar) == 0 {
		for k, _ := range p.fans {
			tar = append(tar, k)
		}
	}

	var t *node
	for _, fid := range tar {
		if _, ok := p.fans[fid]; !ok {
			continue
		}
		if t, err = sg.load(fid); err != nil {
			return
		} else if t != nil {
			p.Add(sg.factory.Friend(t.p))
			t.Add(sg.factory.Friend(p.p))
			success = append(success, fid)
		} else {
			p.Delete(fid)
		}
	}
	return
}

func (sg *Graph) Refuse(uid string, tar ...string) (err error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	var p *node
	if p, err = sg.load(uid); err != nil {
		return
	}
	if len(tar) == 0 {
		for k, _ := range p.fans {
			tar = append(tar, k)
		}
	}

	for _, fid := range tar {
		p.fans.Delete(fid)
		if t := sg.nodes[fid]; t != nil {
			t.follow.Delete(uid)
		}
	}
	return
}

// RangeFriend 遍历我的好友
func (sg *Graph) RangeFriend(uid string, f func(Friend) bool) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return
	}
	for _, v := range p.friends {
		if !f(v) {
			return
		}
	}
}

func (sg *Graph) RangePlayer(uid string, f func(Player) bool) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return
	}
	for _, v := range p.friends {
		if t := sg.nodes[v.GetUid()]; t != nil {
			if !f(t.p) {
				return
			}
		}
	}
}

// Apply 我的粉丝 ，关注我的人，等待申请
//
//	uid --> 申请时间
func (sg *Graph) Apply(uid string, t Friendship) Apply {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	switch t {
	case FriendshipFans:
		return p.fans.Clone()
	case FriendshipFollow:
		return p.follow.Clone()
	default:
		return nil

	}
}

// Friends 获取用户的所有好友
func (sg *Graph) Friends(uid string) []Friend {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	result := make([]Friend, 0, len(p.friends))

	for _, v := range p.friends {
		result = append(result, v)
	}
	return result
}

// Broadcast 好友广播
func (sg *Graph) Broadcast(uid string, name string, data any) {
	fs := sg.Friends(uid)
	for _, u := range fs {
		if t := sg.nodes[u.GetUid()]; t != nil {
			t.p.SendMessage(name, data)
		}
	}
}
