package graph

import (
	"github.com/hwcer/cosgo/utils"
)

// RecommendFilter 推荐用户过滤器，过滤掉最近推荐过的用户
type RecommendFilter func(uid string) bool

// RecommendHandle 需要在recommendAppend 循环调用 RecommendHandle 直到 返回fasle
type RecommendHandle func(uid string) bool

// RecommendAppend 推荐数量不足时，调用 recommendAppend,
type RecommendAppend func(RecommendHandle)

// Recommend 获取好友推荐（共同好友最多的用户）
func (sg *Graph) Recommend(uid string, size int, filter RecommendFilter, done RecommendAppend) []string {
	if size == 0 {
		return nil
	}
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	p := sg.nodes[uid]
	if p == nil {
		return nil
	}
	// 统计共同好友数
	friendUsers := make(map[string]struct{})

	var verifyAndModify = func(t string) (next bool) {
		if t == uid || p.Has(t, RelationFollow+RelationFriend) {
			return true
		}
		if _, ok := friendUsers[t]; ok {
			return true
		}
		if filter != nil && !filter(t) {
			return true
		}
		friendUsers[t] = struct{}{}
		return len(friendUsers) < size
	}
	//共同好友
	for k := range p.friends {
		fd := sg.nodes[k]
		if fd == nil {
			continue
		}
		for potentialFriend := range fd.friends {
			if !verifyAndModify(potentialFriend) {
				return utils.MapKeys(friendUsers)
			}
		}
	}

	//自定义推荐
	if done != nil {
		done(verifyAndModify)
	}

	return utils.MapKeys(friendUsers)
}
