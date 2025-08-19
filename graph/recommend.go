package graph

// RecommendFilter 推荐用户过滤器，过滤掉最近推荐过的用户
type RecommendFilter func(Player) bool

// RecommendHandle 需要在recommendAppend 循环调用 RecommendHandle 直到 返回fasle
type RecommendHandle func(Player) bool

// RecommendAppend 推荐数量不足时，调用 recommendAppend,
type RecommendAppend func(RecommendHandle)

// Recommend 获取好友推荐（共同好友最多的用户）
func (sg *Graph) Recommend(uid string, size int, filter RecommendFilter, done RecommendAppend) map[string]Player {
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
	friendUsers := make(map[string]Player)

	var filterDefault = func(t string) Player {
		if t == uid || p.friends.Has(t) {
			return nil
		}
		if _, ok := friendUsers[t]; ok {
			return nil
		}
		n := sg.nodes[t]
		if n == nil {
			return nil
		}
		if filter != nil && !filter(n.p) {
			return nil
		}

		return n.p
	}
	//共同好友
	for k, _ := range p.friends {
		fd := sg.nodes[k]
		if fd == nil {
			continue
		}
		for potentialFriend, _ := range fd.friends {
			if ff := filterDefault(potentialFriend); ff != nil {
				friendUsers[potentialFriend] = ff
				if len(friendUsers) >= size {
					return friendUsers
				}
			}
		}
	}

	//自定义推荐
	if done != nil {
		done(func(t Player) bool {
			k := t.GetUid()
			if f := filterDefault(k); f != nil {
				friendUsers[k] = t
			}
			return len(friendUsers) < size
		})
	}

	return friendUsers
}
