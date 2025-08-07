package graph

import (
	"sort"
	"sync"

	"github.com/hwcer/cosgo/values"
)

var ErrorUserNotExist = values.Error("user not exist")

// Factory 通过ID生成User
type Factory func(id string) (User, error)

type Install func(user1, user2 User)

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
	i = g.install
	return
}

// 获取，或者创建
func (sg *Graph) load(u User) (p *Player) {
	id := u.GetUid()
	if p = sg.nodes[id]; p == nil {
		p = NewPlayer(u)
		sg.nodes[id] = p
	}
	return
}

func (sg *Graph) create(uid string) (p *Player, err error) {
	if p = sg.nodes[uid]; p == nil {
		var u User
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

// install 初始化好友关系，无锁直接快速创建图谱
func (sg *Graph) install(u1, u2 User) {
	p1 := sg.load(u1)
	p2 := sg.load(u2)
	p1.Add(p2)
	p2.Add(p1)
}

// User 通过ID获取用户对象,可能不存在哦
func (sg *Graph) User(uid string) User {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	if p := sg.nodes[uid]; p != nil {
		return p.User
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

// Get 获取我的好友数据
func (sg *Graph) Get(uid, tar string) User {
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
	return t.User
}

// Follow 关注好友，如果对方也关注自己，则直接成为好友关系
// 同意对方的申请好友时，无脑关注对方就行
func (sg *Graph) Follow(uid, tar string) (err error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	var p, t *Player
	if p, err = sg.create(uid); err != nil {
		return
	}
	if t, err = sg.create(tar); err != nil {
		return
	}
	p.Follow(t)
	return
}

// Delete 移除好友关系
func (sg *Graph) Delete(uid, tar string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	// 双向移除好友关系
	p := sg.nodes[uid]
	if p == nil {
		return
	}
	t := sg.nodes[tar]
	if t == nil {
		return
	}

	p.Delete(t)
}

// Range 遍历我的好友
func (sg *Graph) Range(uid string, f func(User) bool) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return
	}
	for _, v := range p.friends {
		if !f(v.User) {
			return
		}
	}
}

// Fans 我的粉丝 ，关注我的人，等待申请
func (sg *Graph) Fans(uid string) []User {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	result := make([]User, 0, len(p.fans))
	for _, fans := range p.fans {
		result = append(result, fans.User)
	}
	return result
}

// Friends 获取用户的所有好友
func (sg *Graph) Friends(uid string) []User {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	result := make([]User, 0, len(p.friends))
	for _, friend := range p.friends {
		result = append(result, friend.User)
	}
	return result
}

// Recommend 获取好友推荐（共同好友最多的用户）
func (sg *Graph) Recommend(uid string, limit int) []User {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	p := sg.nodes[uid]
	if p == nil || len(p.friends) == 0 {
		return nil
	}

	// 统计共同好友数
	commonCount := make(map[string]int)
	friendUsers := make(map[string]*Player)

	for _, friend := range p.friends {
		//fmt.Println("推荐：我的好友", friend)
		for _, potentialFriend := range friend.friends {
			if potentialID := potentialFriend.GetUid(); potentialID != uid && p.friends[potentialID] == nil {
				commonCount[potentialID]++
				friendUsers[potentialID] = potentialFriend
			}
		}
	}

	if len(commonCount) == 0 {
		return nil
	}

	// 转换为可排序的切片
	type recommendation struct {
		user  User
		count int
	}
	recs := make([]recommendation, 0, len(commonCount))
	for k, count := range commonCount {
		recs = append(recs, recommendation{friendUsers[k], count})
	}

	// 按共同好友数排序
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].count > recs[j].count
	})

	// 提取用户列表
	result := make([]User, 0, len(recs))
	for _, rec := range recs {
		result = append(result, rec.user)
	}

	// 限制返回数量
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result
}
