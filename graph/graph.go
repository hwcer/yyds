package graph

import (
	"sync"

	"github.com/hwcer/cosgo/values"
)

var ErrorUserNotExist = values.Error("user not exist")

// Factory 通过ID生成User
type Factory func(id string) (Data, error)

type Graph struct {
	mu      sync.RWMutex       // 读写锁保证并发安全
	nodes   map[string]*Player // 用户ID到用户对象的映射
	factory Factory            // 用户工厂函数
}

// New 创建一个新的社交图谱
func New(userFactory Factory) (g *Graph, i Install) {
	g = &Graph{
		nodes:   make(map[string]*Player),
		factory: userFactory,
	}
	i = Install{g: g}
	return
}

// 获取，或者创建
func (sg *Graph) load(uid string) (p *Player, err error) {
	if p = sg.nodes[uid]; p == nil {
		var u Data
		if u, err = sg.factory(uid); err != nil {
			return
		} else if u == nil {
			return nil, ErrorUserNotExist
		}
		p = NewPlayer(u)
		sg.nodes[uid] = p
	}
	return
}

// Data 通过ID获取用户对象,可能不存在哦
func (sg *Graph) Data(uid string) Data {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	if p := sg.nodes[uid]; p != nil {
		return p.Data
	}
	return nil
}

// Add 添加新用户
func (sg *Graph) Add(uid string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if _, exists := sg.nodes[uid]; !exists {
		if u, _ := sg.factory(uid); u != nil {
			sg.nodes[uid] = NewPlayer(u)
		}
	}
}
func (sg *Graph) Has(uid, tar string) bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return false
	}
	_, ok := p.friends[tar]

	return ok
}

// Get 获取我的好友数据
func (sg *Graph) Get(uid, tar string) Data {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	t := p.Get(tar)
	if t == nil {
		return nil
	}
	return t.Data
}

// Follow 关注好友，如果对方也关注自己，则直接成为好友关系
// 同意对方的申请好友时，无脑关注对方就行
// fri 直接成为好友
func (sg *Graph) Follow(uid, tar string) (fri bool, err error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	var p, t *Player
	if p, err = sg.load(uid); err != nil {
		return
	}
	if t, err = sg.load(tar); err != nil {
		return
	}
	fri = p.Follow(t)
	return
}

// Delete 移除好友关系
func (sg *Graph) Delete(uid, tar string) Data {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	// 双向移除好友关系
	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	t := p.Get(tar)
	if t == nil {
		return nil
	}
	p.Delete(t)
	return t.Data
}

// Accept 接受好友申请
// 返回 成功加为好友的列表
func (sg *Graph) Accept(uid string, tar ...string) (success []string, err error) {
	if len(tar) == 0 {
		return
	}
	sg.mu.Lock()
	defer sg.mu.Unlock()
	var p *Player
	if p, err = sg.load(uid); err != nil {
		return
	}
	for _, t := range tar {
		if f, ok := p.fans[t]; ok {
			p.Add(f)
			success = append(success, t)
		}
	}

	return
}

func (sg *Graph) Refuse(uid string, tar ...string) (err error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	var p *Player
	if p, err = sg.load(uid); err != nil {
		return
	}
	if len(tar) == 0 {
		p.fans = map[string]*Player{}
		return
	}
	for _, t := range tar {
		delete(p.fans, t)
	}
	return

}

// Range 遍历我的好友
func (sg *Graph) Range(uid string, f func(Data) bool) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return
	}
	for _, v := range p.friends {
		if !f(v.Data) {
			return
		}
	}
}

// Fans 我的粉丝 ，关注我的人，等待申请
func (sg *Graph) Fans(uid string) []Data {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	result := make([]Data, 0, len(p.fans))
	for _, fans := range p.fans {
		result = append(result, fans.Data)
	}
	return result
}

// Friends 获取用户的所有好友
func (sg *Graph) Friends(uid string) []Data {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	result := make([]Data, 0, len(p.friends))
	for _, friend := range p.friends {
		result = append(result, friend.Data)
	}
	return result
}

// Broadcast 好友广播
func (sg *Graph) Broadcast(uid string, name string, data any) {
	fs := sg.Friends(uid)
	for _, u := range fs {
		u.SendMessage(name, data)
	}
}

// RecommendFilter 推荐用户过滤器，过滤掉最近推荐过的用户
type RecommendFilter func(tar Data) bool

// RecommendHandle 需要在recommendAppend 循环调用 RecommendHandle 直到 返回fasle
type RecommendHandle func(tar Data) bool

// RecommendAppend 推荐数量不足时，调用 recommendAppend,
type RecommendAppend func(RecommendHandle)

// Recommend 获取好友推荐（共同好友最多的用户）
func (sg *Graph) Recommend(uid string, size int, filter RecommendFilter, done RecommendAppend) map[string]Data {
	if size == 0 {
		return nil
	}
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	p := sg.nodes[uid]
	if p == nil || len(p.friends) == 0 {
		return nil
	}
	// 统计共同好友数
	//commonCount := make(map[string]int)
	friendUsers := make(map[string]Data)

	var filterDefault = func(tar Data) bool {
		t := tar.GetUid()
		if t == uid || p.friends.Has(t) {
			return false
		}
		if _, ok := friendUsers[t]; ok {
			return false
		}
		if filter != nil && !filter(tar) {
			return false
		}

		return true
	}
	var l int
	//优先推荐粉丝
	for k, v := range p.fans {
		if filterDefault(v) {
			l++
			friendUsers[k] = v.Data
			if l >= size {
				return friendUsers
			}
		}
	}
	//共同好友
	for _, friend := range p.friends {
		for _, potentialFriend := range friend.friends {
			if filterDefault(potentialFriend) {
				l++
				friendUsers[potentialFriend.GetUid()] = potentialFriend.Data
				if l >= size {
					return friendUsers
				}
			}
		}
	}

	//随机推荐
	for k, v := range sg.nodes {
		if filterDefault(v) {
			l++
			friendUsers[k] = v.Data
			if l >= size {
				return friendUsers
			}
		}
	}
	//自定义推荐
	if done != nil {
		done(func(tar Data) bool {
			if filterDefault(tar) {
				l++
				friendUsers[tar.GetUid()] = tar
			}
			return l < size
		})
	}

	return friendUsers
}
