package graph

type Friendship int8

const (
	FriendshipNone   Friendship = 0
	FriendshipFans              = 1 //我的粉丝
	FriendshipFollow            = 2 //我的关注
	FriendshipFriend            = 3 //我的好友
)

func newNode(p Player) *Node {
	return &Node{p: p, fans: Apply{}, follow: Apply{}, friends: relation{}}
}

type Node struct {
	p       Player
	fans    Apply    //我的粉丝
	follow  Apply    //我关注的人
	friends relation //好友关系
}

// Has 0- 无关系，1-我的粉丝，2-好友关系
func (p *Node) Has(uid string) Friendship {
	if p.friends.Has(uid) {
		return FriendshipFriend
	} else if p.follow.Has(uid) {
		return FriendshipFollow
	} else if p.fans.Has(uid) {
		return FriendshipFans
	} else {
		return FriendshipNone
	}
}

// Get 获取我的好友信息
func (p *Node) Get(fid string) Friend {
	return p.friends[fid]
}

// Add 添加好友
func (p *Node) Add(fd Friend) {
	id := fd.GetUid()
	p.friends.Add(fd)
	p.fans.Delete(id)
	p.follow.Delete(id)
}

// Fans 添加粉丝
func (p *Node) Fans(id string) {
	p.fans.Add(id)
}

// Follow 添加关注
func (p *Node) Follow(id string) {
	p.follow.Add(id)
}

// Delete 删除好友
func (p *Node) Delete(id string) Friend {
	r := p.friends[id]
	if r != nil {
		p.friends.Delete(id)
	}
	return r
}
