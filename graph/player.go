package graph

type Friendship int8

const (
	FriendshipNone   Friendship = 0
	FriendshipFans              = 1 //我的粉丝
	FriendshipFriend            = 2 //我的好友
)

type Data interface {
	GetUid() string
	SendMessage(name string, data any)
}

func NewPlayer(d Data) *Player {
	return &Player{Data: d, fans: relation{}, friends: relation{}}
}

type relation map[string]*Player

func (r relation) Has(fid string) bool {
	_, ok := r[fid]
	return ok
}

func (r relation) Add(p *Player) {
	r[p.GetUid()] = p
}

func (r relation) Del(fid string) {
	delete(r, fid)
}

type Player struct {
	Data
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
func (p *Player) Get(fid string) *Player {
	return p.friends[fid]
}

// Add 添加好友
func (p *Player) Add(tar *Player) {
	p.friends.Add(tar)
	p.fans.Del(tar.GetUid())
}

// Delete 删除好友
func (p *Player) Delete(tar *Player) {
	p.friends.Del(tar.GetUid())
	tar.friends.Del(p.GetUid())
}

func (p *Player) Follow(tar *Player) (friend bool) {
	fid := tar.GetUid()
	if _, exist := p.fans[fid]; exist {
		//直接成为好友
		friend = true
		p.friends.Add(tar)
		tar.friends.Add(p)
		p.fans.Del(p.GetUid())
	} else {
		//给对方推送一个粉丝
		tar.fans.Add(p)
	}
	return
}
