package dataset

import (
	"fmt"
	"testing"
)

// 定义一个结构体
type ExampleStruct struct {
	Field1 []int
	Field2 string
}

func TestDocument(t *testing.T) {
	src := &ExampleStruct{Field1: []int{1}, Field2: "test"}
	doc := NewDoc(src)
	copied := doc.Clone()

	e := copied.Any().(*ExampleStruct)
	fmt.Println("src:", src.Field1, src.Field2)
	fmt.Println("copied:", e.Field1, e.Field2)

	e.Field1[0] = 2
	e.Field2 = "test2"
	fmt.Println("src:", src.Field1, src.Field2)
	fmt.Println("copied:", e.Field1, e.Field2)
}
