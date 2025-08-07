package graph

type Relation int8

const (
	RelationNone   Relation = 0
	RelationFollow          = 1 //我的关注，我的女神，我的白月光
	RelationFriend          = 2 //我的好友
)

func NewPlayer(u User) *Player {
	return &Player{User: u, friends: relation{}, fans: relation{}}
}

// User 定义用户接口
type User interface {
	GetUid() string
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

// Has 0- 无关系，1-关注了我，2-好友关系
func (p *Player) Has(uid string) Relation {
	if p.friends.Has(uid) {
		return RelationFriend
	} else if p.fans.Has(uid) {
		return RelationFollow
	} else {
		return RelationNone
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

// Follow 我关注 tar,成为他的粉丝
func (p *Player) Follow(tar *Player) {
	if _, exist := p.fans[tar.GetUid()]; exist {
		//直接成为好友
		p.friends.Add(tar)
		tar.friends.Add(p)
		p.fans.Delete(p)
	} else {
		//给对方推送一个粉丝
		tar.fans.Add(p)
	}

}
