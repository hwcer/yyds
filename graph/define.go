package graph

import (
	"time"

	"github.com/hwcer/cosgo/values"
)

var ErrorUserNotExist = values.Error("user not exist")

// Factory 通过ID生成User
type Factory interface {
	Player(string) (Player, error)
	Friend(Player) Friend
}

// Player 用户信息
type Player interface {
	GetUid() string
	SendMessage(name string, data any)
}

// Friend 好友信息
type Friend interface {
	GetUid() string
}

// /////////////////////////////////////////////////////////////////////////
type Apply map[string]int64

func (this Apply) Has(id string) bool {
	_, ok := this[id]
	return ok
}

func (this Apply) Add(id string) {
	this[id] = time.Now().Unix()
}
func (this Apply) Delete(id string) {
	delete(this, id)
}
func (this Apply) Clone() Apply {
	r := make(Apply, len(this))
	for k, v := range this {
		r[k] = v
	}
	return r

}

// /////////////////////////////////////////////////////////////////////////
type relation map[string]Friend

func (this relation) Has(id string) bool {
	_, ok := this[id]
	return ok
}
func (this relation) Add(fd Friend) {
	id := fd.GetUid()
	this[id] = fd
}
func (this relation) Delete(id string) {
	delete(this, id)
}
