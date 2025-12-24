package graph

import (
	"sync"
)

type Graph struct {
	mu      sync.RWMutex       // 读写锁保证并发安全
	nodes   map[string]*Player // 用户ID到用户对象的映射
	factory Factory            // 用户工厂函数
	Limit   int32              //好友上限
}

// New 创建一个新的社交图谱
func New(factory Factory) (g *Graph, i Install) {
	g = &Graph{
		nodes:   make(map[string]*Player),
		factory: factory,
	}
	i = Install{g: g}
	return
}

// 获取，或者创建
func (sg *Graph) load(uid string) (*Player, error) {
	r := sg.nodes[uid]
	if r == nil {
		if v, err := sg.factory.Create(uid); err != nil {
			return nil, err
		} else {
			r = NewPlayer(uid, v)
			sg.nodes[uid] = r
		}

	}
	return r, nil
}

// Add 添加新用户
func (sg *Graph) Add(uid string) error {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	if _, exists := sg.nodes[uid]; !exists {
		if v, err := sg.factory.Create(uid); err == nil {
			sg.nodes[uid] = NewPlayer(uid, v)
		} else {
			return err
		}
	}
	return nil
}

// Has 是否有好友关系
func (sg *Graph) Has(uid, fid string) bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	n := sg.nodes[uid]
	if n == nil {
		return false
	}
	v, ok := n.friends[fid]
	if !ok {
		return false
	}
	return v.relation.Has(RelationFriend)
}

//
//func (sg *Graph) isMax(uid ...string) bool {
//	if sg.Limit <= 0 {
//		return false
//	}
//	for _, u := range uid {
//		if sg.count(u, RelationFriend) >= sg.Limit {
//			return true
//		}
//	}
//	return false
//}
//
//func (sg *Graph) count(uid string, t Relation) (r int32) {
//	p := sg.nodes[uid]
//	if p == nil {
//		return
//	}
//	for _, friend := range p.friends {
//		if friend.relation.Has(t) {
//			r++
//		}
//	}
//	return
//}

func (sg *Graph) Count(uid []string, t Relation) map[string]int32 {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	r := make(map[string]int32)
	for _, u := range uid {
		if p, _ := sg.load(u); p != nil {
			r[u] = p.Count(t)
		} else {
			r[u] = 0
		}
	}
	return r
}

// Relation 二人关系
func (sg *Graph) Relation(uid, fid string) Relation {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	n := sg.nodes[uid]
	if n == nil {
		return RelationNone
	}
	v, ok := n.friends[fid]
	if !ok {
		return RelationNone
	}
	return v.relation
}

// Follow 关注好友，
// 如果对方也关注自己，则直接成为好友关系
// true 直接成为好友
//
//	结果中 0:粉丝，1:直接成为好友,-1失败，对方已经申请
func (sg *Graph) Follow(uid string, fid []string) (map[string]FollowResult, error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	p, err := sg.load(uid)
	if err != nil {
		return nil, err
	}
	auto := !p.IsMax(sg.Limit)
	r := map[string]FollowResult{}
	for _, f := range fid {
		r[f] = sg.follow(p, f, auto)
	}
	return r, nil
}

func (sg *Graph) follow(p *Player, fid string, auto bool) FollowResult {
	//查询对方是我的粉丝
	relation := p.Relation(fid)
	if relation.Has(RelationUnfriend) {
		return FollowResultUnfriend //被拉黑
	} else if relation.Has(RelationFriend) {
		return FollowResultFriend //已经是好友
	} else if relation.Has(RelationFollow) {
		return FollowResultNone //已经申请过
	}
	t, err := sg.load(fid)
	if err != nil {
		return FollowResultFailure
	}

	uid := p.Uid()
	//对方拉黑了你
	if t.Has(uid, RelationUnfriend) {
		return FollowResultUnfriend
	}

	r := FollowResultNone
	if relation.Has(RelationFans) && auto && !t.IsMax(sg.Limit) {
		//我的粉丝并且双方好友未满直接成为好友
		r = FollowResultFriend
		_ = p.Modify(fid, RelationFriend)
		_ = t.Modify(uid, RelationFriend)
	} else {
		_ = p.Modify(fid, RelationFollow)
		_ = t.Modify(uid, RelationFans)
	}

	return r
}

// Remove 移除好友关系
func (sg *Graph) Remove(uid string, fid []string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	for _, k := range fid {
		sg.remove(uid, k)
	}
}

func (sg *Graph) remove(uid, fid string) {
	// 双向移除好友关系
	if p := sg.nodes[uid]; p != nil {
		p.Remove(fid)
	}
	if p := sg.nodes[fid]; p != nil {
		p.Remove(uid)
	}
}

// Unfriend 拉黑，阻止对方加我好友
func (sg *Graph) Unfriend(uid, fid string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	// 双向拉黑
	if p, err := sg.load(uid); err == nil {
		p.Unfriend(fid)
	}
	if p, err := sg.load(fid); err == nil {
		p.Unfriend(uid)
	}
}

func (sg *Graph) accept(p *Player, fid string, fast bool) (success bool, err error) {

	if f := p.friends[fid]; f == nil && !fast {
		return false, nil
	} else if f != nil && f.relation != RelationFans {
		return false, nil
	}
	var t *Player
	t, err = sg.load(fid)
	if err != nil {
		return false, err
	}
	if t.IsMax(sg.Limit) {
		if fast {
			sg.follow(p, fid, false)
		} else {
			err = ErrorTargetFriendMax
		}
		return
	}
	p.Modify(fid, RelationFriend)
	t.Modify(p.Uid(), RelationFriend)
	return true, nil
}

// Accept 接受好友申请
// 返回 成功加为好友的列表
// fast 快速成为好友，不需要先申请,如果好友已满自动申请
// fast 模式不会返回错误，除非用户ID 不存在
func (sg *Graph) Accept(uid string, tar []string, fast bool) (success []string, err error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	var p *Player
	if p, err = sg.load(uid); err != nil {
		return
	}

	if p.IsMax(sg.Limit) {
		if fast {
			for _, t := range tar {
				sg.follow(p, t, false)
			}
		} else {
			return nil, ErrorYourFriendMax
		}
	}

	if len(tar) == 0 {
		for k, v := range p.friends {
			if v.relation == RelationFans {
				tar = append(tar, k)
			}
		}
	}

	for _, fid := range tar {
		if s, e := sg.accept(p, fid, fast); e != nil {
			return success, e
		} else if s {
			success = append(success, fid)
		}
	}

	return
}

func (sg *Graph) Refuse(uid string, tar []string) (err error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	var p *Player
	if p, err = sg.load(uid); err != nil {
		return
	}
	if len(tar) == 0 {
		for k, v := range p.friends {
			if v.relation == RelationFans {
				tar = append(tar, k)
			}
		}
	}

	for _, fid := range tar {
		if f := p.friends[fid]; f == nil || f.relation != RelationFans {
			continue
		}
		p.Remove(fid)
		if t := sg.nodes[fid]; t != nil {
			t.Remove(uid)
		}
	}
	return
}

// Range 遍历我的好友个人信息
// 请勿将values 使用在回调函数作用域以外的地方
func (sg *Graph) Range(uid string, relation Relation, handle func(Getter) bool) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	p := sg.nodes[uid]
	if p == nil {
		return
	}
	for fid, fri := range p.friends {
		if !fri.Has(relation) {
			continue
		}
		g := Getter{sg: sg, fid: fid, fri: fri}
		if !handle(g) {
			break
		}
	}
}

// Lock 获取读写锁，在所内操作数据
// 禁止将Player Friend Friend.Values Friend.Values 在回调函数作用域以外使用
func (sg *Graph) Lock(handle func(Statement)) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	handle(Statement{g: sg})
}

// RLock 获取只读锁，在锁内读数据
// 禁止将Player Friend Friend.Values Friend.Values 在回调函数作用域以外使用
func (sg *Graph) RLock(handle func(Statement)) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	handle(Statement{g: sg})
}

// Modify 获取修改用户缓存信息
// 请勿将Player 使用在回调函数作用域以外的地方
// 返回是否成功标记
func (sg *Graph) Modify(uid string, handle func(*Player) error) error {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	if p, err := sg.load(uid); err == nil {
		return handle(p)
	} else {
		return err
	}
}

// Reader 获取修改用户缓存信息
// 请勿将Player 使用在回调函数作用域以外的地方
// 请勿修改任何数据
func (sg *Graph) Reader(uid string, handle func(*Player)) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	if p := sg.nodes[uid]; p != nil {
		handle(p)
	}
}

// Broadcast 好友广播
func (sg *Graph) Broadcast(uid string, name string, data any) {
	p := sg.nodes[uid]
	if p == nil {
		return
	}
	var fs []string
	sg.mu.RLock()
	for k, v := range p.friends {
		if v.Has(RelationFriend) {
			fs = append(fs, k)
		}
	}
	sg.mu.RUnlock()

	for _, k := range fs {
		sg.factory.SendMessage(k, name, data)
	}
}
