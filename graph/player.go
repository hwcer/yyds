package graph

type Friendship int8

const (
	FriendshipNone   Friendship = 0
	FriendshipFans              = 1 //我的粉丝
	FriendshipFriend            = 2 //我的好友
)

func NewPlayer(u User) *Player {
	return &Player{User: u, friends: relation{}, fans: relation{}}
}

// User 定义用户接口
type User interface {
	GetUid() string
	SendMessage(name, data any)
}

type relation map[string]*Player

func (r relation) Has(uid string) bool {
	_, ok := r[uid]
	return ok
}

func (r relation) Add(tar *Player) {
	r[tar.GetUid()] = tar
}

func (r relation) Delete(tar *Player) {
	delete(r, tar.GetUid())
}

type Player struct {
	User
	fans    relation //我的粉丝，等待我批准成为好友
	friends relation //好友关系
}

// Has 0- 无关系，1-我的粉丝，2-好友关系
func (p *Player) Has(uid string) Friendship {
	if p.friends.Has(uid) {
		return FriendshipFriend
	} else if p.fans.Has(uid) {
		return FriendshipFans
	} else {
		return FriendshipNone
	}
}

// Get 获取我的好友信息
func (p *Player) Get(uid string) *Player {
	return p.friends[uid]
}

// Add 添加好友
func (p *Player) Add(tar *Player) {
	p.friends.Add(tar)
}

// Delete 删除好友,双向删除
func (p *Player) Delete(tar *Player) {
	p.friends.Delete(tar)
	tar.friends.Delete(p)
}

func (p *Player) Follow(tar *Player) (friend bool) {
	if _, exist := p.fans[tar.GetUid()]; exist {
		//直接成为好友
		friend = true
		p.friends.Add(tar)
		tar.friends.Add(p)
		p.fans.Delete(p)
	} else {
		//给对方推送一个粉丝
		tar.fans.Add(p)
	}
	return
}
