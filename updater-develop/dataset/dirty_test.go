package dataset

import (
	"fmt"
	"testing"
)

type role struct {
	Id   string
	Lv   int32 `json:"lv" bson:"lv"`
	Name string
}

func TestName(t *testing.T) {
	player := NewColl()
	for i := int32(1); i < 100; i++ {
		id := fmt.Sprintf("%v", i)
		player.Receive(id, &role{
			Id:   id,
			Lv:   i,
			Name: "Name-" + id,
		})
	}
	k := "1"
	doc, _ := player.Get(k)
	t.Logf("%+v", doc.Any())
	if err := player.Update(k, Update{"lv": 100}); err != nil {
		t.Logf("Update Err:%v", err)
	}
	t.Logf("lv:%v", doc.Val("lv"))
	if err := player.Save(nil); err != nil {
		t.Logf("Save Err:%v", err)
	}
	t.Logf("修改结果：%+v", doc.Any())
	player.Release()
}
