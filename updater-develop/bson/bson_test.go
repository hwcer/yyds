package bson

import (
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"testing"
)

type bsonPlayer struct {
	Id    string
	Name  string
	Lv    int32
	Info  bsonInfo
	Items []int32
}

type bsonInfo struct {
	Vip  int32
	Desc string
}

var testPlayer = &bsonPlayer{
	Id:    "1",
	Name:  "hwc",
	Lv:    100,
	Items: []int32{1, 2},
}

var doc Document
var bsonBytes []byte

func init() {
	doc, _ = Marshal(testPlayer)
	bsonBytes = doc.Bytes(nil)
}
func TestNew(t *testing.T) {
	t.Logf("bsonBytes len:%v", len(bsonBytes))
	t.Logf("Document len:%v", doc.Len())

	_ = doc.Set("info.vip", 100)
	if err := doc.Set("items.20", 20); err != nil {
		t.Logf("Error:%v", err)
	}

	if r, err := doc.Inc("info.vip", 2); err != nil {
		t.Logf("INC Error:%v", err)
	} else {
		t.Logf("INC result:%v", r)
	}

	t.Logf("Document:%v", doc.String())

	t.Logf("info.vip:%v", doc.GetInt32("info.vip"))
	t.Logf("items.20:%v", doc.GetInt32("items.20"))

	newPlayer := &bsonPlayer{}
	if err := doc.Unmarshal(newPlayer); err != nil {
		t.Logf("Unmarshal Error:%v", err)
	} else {
		t.Logf("newPlayer:%+v", newPlayer)
	}
}

func BenchmarkBsonSet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = doc.Set("info.vip", 100)
	}

}
func BenchmarkBsonGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = doc.GetInt32("info.vip")
	}
}

func BenchmarkBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = doc.Bytes(nil)
	}
}

func BenchmarkMetaGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d := bsoncore.Document(bsonBytes)
		v := d.Lookup("info.vip")
		_, _ = v.Int32OK()
	}
}
