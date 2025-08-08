package graph

import (
	"fmt"
	"testing"
	"unsafe"
)

var g *Graph

func init() {
	var init Install
	g, init = New(_factory)

	for i := 1; i <= 200000; i++ {
		u, _ := _factory(fmt.Sprintf("%d", i))
		for j := 1; j <= 100; j++ {
			f, _ := _factory(fmt.Sprintf("%d", i+j))
			init(u, f)
		}
	}
}

func TestName(t *testing.T) {
	g.PrintMemoryUsage()
}

func TestGraph_Recommend(t *testing.T) {
	uid := "100"
	for _, u := range g.Recommend(uid, 10) {
		fmt.Println("推荐好友", *u.(*_user))
	}
}

type _user struct {
	Id   string
	Name string
	Icon string
}

func (this *_user) GetUid() string {
	return this.Id
}
func (this *_user) SendMessage(name, data any) {

}
func (this *_user) MemoryUsage() uintptr {
	size := unsafe.Sizeof(*this)    // 结构体基础大小
	size += uintptr(len(this.Id))   // string 数据
	size += uintptr(len(this.Name)) // string 数据
	size += uintptr(len(this.Icon)) // string 数据
	return size
}

func _factory(id string) (User, error) {
	return &_user{Id: id, Name: "foo", Icon: "KaiXin"}, nil
}
