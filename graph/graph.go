package graph

import (
	"container/list"
	"sort"
	"sync"
)

// User 定义用户接口
type User interface {
	GetUid() string
}
type UserMemoryUsage interface {
	MemoryUsage() uintptr
}

// Factory 通过ID生成User
type Factory func(id string) (User, error)

type Install func(user1, user2 User)

// Graph 表示社交图谱结构
type Graph struct {
	mu          sync.RWMutex               // 读写锁保证并发安全
	nodes       map[string]User            // 用户ID到用户对象的映射
	friends     map[string]map[string]User // 邻接表表示的好友关系 (ID -> ID -> User)
	userFactory Factory                    // 用户工厂函数
}

// New 创建一个新的社交图谱
func New(userFactory Factory) (g *Graph, i Install) {
	g = &Graph{
		nodes:       make(map[string]User),
		friends:     make(map[string]map[string]User),
		userFactory: userFactory,
	}
	i = g.install
	return
}

// 初始化，无锁直接快速创建图谱
func (sg *Graph) install(user1, user2 User) {
	id1 := user1.GetUid()
	id2 := user2.GetUid()

	if _, ok := sg.nodes[id1]; !ok {
		sg.nodes[id1] = user1
		sg.friends[id1] = make(map[string]User)
	}

	if _, ok := sg.nodes[id2]; !ok {
		sg.nodes[id2] = user2
		sg.friends[id2] = make(map[string]User)
	}

	sg.friends[id1][id2] = sg.nodes[id2]
	sg.friends[id2][id1] = sg.nodes[id1]
}

// GetUser 通过ID获取用户对象,可能不存在哦
func (sg *Graph) GetUser(id string) User {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	return sg.nodes[id]
}

// AddUser 添加新用户
func (sg *Graph) AddUser(user User) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	id := user.GetUid()
	if _, exists := sg.nodes[id]; !exists {
		sg.nodes[id] = user
		sg.friends[id] = make(map[string]User)
	}
}

// GetFriend 获取特定好友
func (sg *Graph) GetFriend(id1, id2 string) User {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	if friends := sg.friends[id1]; friends != nil {
		return friends[id2]
	}
	return nil
}

// AddFriend 添加好友关系，如果用户不存在则自动创建
func (sg *Graph) AddFriend(id1, id2 string) (err error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// 确保用户1存在
	if _, exists := sg.nodes[id1]; !exists {
		if sg.nodes[id1], err = sg.userFactory(id1); err != nil {
			return
		}
		sg.friends[id1] = make(map[string]User)
	}

	// 确保用户2存在
	if _, exists := sg.nodes[id2]; !exists {
		if sg.nodes[id2], err = sg.userFactory(id2); err != nil {
			return
		}
		sg.friends[id2] = make(map[string]User)
	}

	// 双向添加好友关系
	sg.friends[id1][id2] = sg.nodes[id2]
	sg.friends[id2][id1] = sg.nodes[id1]
	return
}

// RemoveFriend 移除好友关系
func (sg *Graph) RemoveFriend(id1, id2 string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	// 双向移除好友关系
	if _, ok := sg.friends[id1]; !ok {
		delete(sg.friends[id1], id2)
	}
	if _, ok := sg.friends[id2]; !ok {
		delete(sg.friends[id2], id1)
	}
}

func (sg *Graph) Range(id string, f func(User) bool) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	friends := sg.friends[id]
	if friends == nil {
		return
	}
	for _, u := range friends {
		if !f(u) {
			return
		}
	}
}

// GetFriends 获取用户的所有好友
func (sg *Graph) GetFriends(id string) []User {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	if friends, exists := sg.friends[id]; exists {
		result := make([]User, 0, len(friends))
		for _, friend := range friends {
			result = append(result, friend)
		}
		return result
	}
	return nil
}

// Recommend 获取好友推荐（共同好友最多的用户）
func (sg *Graph) Recommend(id string, limit int) []User {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	currentFriends, exists := sg.friends[id]
	if !exists || len(currentFriends) == 0 {
		return nil
	}

	// 统计共同好友数
	commonCount := make(map[string]int)
	friendUsers := make(map[string]User)

	for _, friend := range currentFriends {
		friendID := friend.GetUid()
		//fmt.Println("推荐：我的好友", friend)
		for _, potentialFriend := range sg.friends[friendID] {
			potentialID := potentialFriend.GetUid()
			// 排除已经是好友的和自己
			if potentialID != id && sg.friends[id][potentialID] == nil {
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

// Shortcut 计算两个用户之间的最短路径（BFS实现）
func (sg *Graph) Shortcut(sourceID, targetID string) []User {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	if _, exists := sg.nodes[sourceID]; !exists {
		return nil
	}
	if _, exists := sg.nodes[targetID]; !exists {
		return nil
	}

	// BFS队列
	queue := list.New()
	queue.PushBack(sourceID)

	// 记录访问路径
	visited := make(map[string]bool)
	visited[sourceID] = true

	// 记录路径
	prev := make(map[string]string)

	found := false
	for queue.Len() > 0 {
		current := queue.Remove(queue.Front()).(string)

		if current == targetID {
			found = true
			break
		}

		for friendID := range sg.friends[current] {
			if !visited[friendID] {
				visited[friendID] = true
				prev[friendID] = current
				queue.PushBack(friendID)
			}
		}
	}

	if !found {
		return nil
	}

	// 重构路径
	var pathIDs []string
	at := targetID
	for at != sourceID {
		pathIDs = append(pathIDs, at)
		at = prev[at]
	}
	pathIDs = append(pathIDs, sourceID)

	// 反转路径
	for i, j := 0, len(pathIDs)-1; i < j; i, j = i+1, j-1 {
		pathIDs[i], pathIDs[j] = pathIDs[j], pathIDs[i]
	}

	// 转换为User对象
	path := make([]User, len(pathIDs))
	for i, id := range pathIDs {
		path[i] = sg.nodes[id]
	}

	return path
}
