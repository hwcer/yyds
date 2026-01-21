package graph

import (
	"sync"
)

type Graph struct {
	mu      sync.RWMutex       // 读写锁保证并发安全
	nodes   map[string]*Player // 用户ID到用户对象的映射
	Factory Factory            // 用户工厂函数
}

// New 创建一个新的社交图谱
func New(factory Factory) (g *Graph, i Install) {
	g = &Graph{
		nodes:   make(map[string]*Player),
		Factory: factory,
	}
	i = Install{g: g}
	return
}

// 获取，或者创建
func (sg *Graph) load(uid string) (*Player, error) {
	r := sg.nodes[uid]
	if r == nil {
		v, err := sg.Factory.Create(uid)
		if err != nil {
			return nil, err
		}
		r = NewPlayer(uid, v)
		sg.nodes[uid] = r
	}
	return r, nil
}

// Add 添加新用户
func (sg *Graph) Add(uid string) error {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	if _, exists := sg.nodes[uid]; !exists {
		if v, err := sg.Factory.Create(uid); err == nil {
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
func (sg *Graph) Follow(uid string, fid []string) *Result {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	p, err := sg.load(uid)
	if err != nil {
		return NewResultError(err)
	}
	auto := !p.IsMax(sg.Factory.Limit(uid))
	r := NewResult()
	for _, f := range fid {
		r.Result[f] = sg.follow(p, f, auto)
	}
	return r
}

func (sg *Graph) follow(p *Player, fid string, auto bool) error {
	if p.uid == fid {
		return ErrorAddYourself
	}
	//查询对方是我的粉丝
	relation := p.Relation(fid)
	if relation.Has(RelationUnfriend) {
		return ErrorYourUnfriend //被拉黑
	} else if relation.Has(RelationFriend) {
		return ErrorAlreadyFriend //已经是好友
	} else if relation.Has(RelationFollow) {
		return ErrorAlreadyFollow //已经申请过
	}
	t, err := sg.load(fid)
	if err != nil {
		return err
	}

	uid := p.Uid()
	//对方拉黑了你
	if t.Has(uid, RelationUnfriend) {
		return ErrorTargetUnfriend
	}

	if relation.Has(RelationFans) && auto && !t.IsMax(sg.Factory.Limit(fid)) {
		//我的粉丝并且双方好友未满直接成为好友
		_ = p.Modify(fid, RelationFriend)
		_ = t.Modify(uid, RelationFriend)
	} else {
		_ = p.Modify(fid, RelationFollow)
		_ = t.Modify(uid, RelationFans)
	}

	return nil
}

// Remove 移除好友关系
func (sg *Graph) Remove(uid string, fid []string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	for _, k := range fid {
		if k != uid {
			sg.remove(uid, k)
		}
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
	if uid == fid {
		return
	}
	sg.mu.Lock()
	defer sg.mu.Unlock()
	if p, err := sg.load(uid); err == nil {
		p.Unfriend(fid)
	}
	//从对方好友列表中移除
	if p, err := sg.load(fid); err == nil {
		p.Remove(uid)
	}
}

func (sg *Graph) accept(p *Player, fid string) error {
	if p.uid == fid {
		return ErrorAddYourself
	}
	if f := p.friends[fid]; f == nil {
		return ErrorUserNotExist
	} else if f.relation.Has(RelationUnfriend) {
		return ErrorYourUnfriend
	} else if f.relation != RelationFans {
		return ErrorNotFans
	}

	t, err := sg.load(fid)
	if err != nil {
		return err
	}

	if relation := t.Relation(p.Uid()); relation.Has(RelationUnfriend) {
		return ErrorTargetUnfriend
	}

	if t.IsMax(sg.Factory.Limit(fid)) {
		return ErrorTargetFriendMax
	}
	p.Modify(fid, RelationFriend)
	t.Modify(p.Uid(), RelationFriend)
	return nil
}

// Accept 接受好友申请
// 返回 成功加为好友的列表
// fast 快速成为好友，不需要先申请,如果好友已满自动申请
// fast 模式不会返回错误，除非用户ID 不存在
func (sg *Graph) Accept(uid string, tar []string) *Result {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	p, err := sg.load(uid)
	if err != nil {
		return NewResultError(err)
	}

	if p.IsMax(sg.Factory.Limit(uid)) {
		return NewResultError(ErrorYourFriendMax)
	}
	//一键通过
	if len(tar) == 0 {
		for k, v := range p.friends {
			if v.relation == RelationFans {
				tar = append(tar, k)
			}
		}
	}
	result := NewResult()

	for _, fid := range tar {
		result.Result[fid] = sg.accept(p, fid)
	}

	return result
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
	// 1. 采用读锁替代写锁，减少锁竞争
	sg.mu.RLock()
	defer sg.mu.RUnlock()
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
func (sg *Graph) Lock(handle func()) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	handle()
}

// Modify 获取修改用户缓存信息
// 请勿将 Player 使用在回调函数作用域以外的地方
// 按照uid数量多次调用 handle
func (sg *Graph) Modify(uid []string, handle func(*Player)) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	for _, u := range uid {
		if p := sg.nodes[u]; p != nil {
			handle(p)
		}
	}
}

// Reader 获取用户缓存信息
// 请勿将Player 使用在回调函数作用域以外的地方
// 按照uid数量多次调用 handle
func (sg *Graph) Reader(uid []string, handle func(Reader)) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	for _, u := range uid {
		if p := sg.nodes[u]; p != nil {
			r := Reader{sg: sg, p: p}
			handle(r)
		}
	}
}

// Broadcast 好友广播
func (sg *Graph) Broadcast(uid string, name string, data any) {
	sg.mu.RLock()
	p := sg.nodes[uid]
	if p == nil {
		sg.mu.RUnlock()
		return
	}
	var fs []string
	for k, v := range p.friends {
		if v.Has(RelationFriend) {
			fs = append(fs, k)
		}
	}
	sg.mu.RUnlock()

	for _, k := range fs {
		sg.Factory.SendMessage(k, name, data)
	}
}

// SetPlayer 设置玩家信息
func (sg *Graph) SetPlayer(uid string, key string, value any) {
	sg.mu.Lock()
	p := sg.nodes[uid]
	if p != nil {
		p.Set(key, value)
	}
	sg.mu.Unlock()
	sg.Factory.SetPlayer(uid, key, value)
}

// SetFriend 设置好友信息，可以临时关系(RelationNone)
// 临时关系 不会调用持久化接口
func (sg *Graph) SetFriend(uid string, fid string, relation Relation, key string, value any) {
	cr := RelationNone
	sg.mu.Lock()
	p := sg.nodes[uid]
	if p != nil {
		fri := p.Friend(fid, relation == RelationNone)
		if fri != nil && fri.Has(relation) {
			cr = fri.relation
			fri.Set(key, value)
		}
	}
	sg.mu.Unlock()
	if cr != RelationNone {
		sg.Factory.SetFriend(uid, fid, key, value)
	}
}
