package bson

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

type Element struct {
	key string
	val bsoncore.Value
	arr *Array   //仅当type == bsontype.Array 时有效
	doc Document //仅当type == bsontype.EmbeddedDocument 时有效
}

func (ele *Element) IsNil() bool {
	return ele.val.Type == 0
}

func (ele *Element) Len() (r int) {
	r += len(ele.key) + 2 //Header  type + key + 0x00
	if ele.val.Type == bsontype.Array {
		r += ele.arr.Len()
	} else if ele.val.Type == bsontype.EmbeddedDocument {
		r += ele.doc.Len()
	} else {
		r += len(ele.val.Data) //data
	}
	return
}

func (ele *Element) Key() string {
	return ele.key
}
func (ele *Element) Type() bsontype.Type {
	return ele.val.Type
}

func (ele *Element) Reset(raw bsoncore.Value) (err error) {
	if raw.Type == bsontype.Array {
		arr := NewArray()
		if err = arr.Reset(raw.Data); err == nil {
			ele.arr = arr
		}
	} else if raw.Type == bsontype.EmbeddedDocument {
		doc := New()
		if err = doc.Reset(raw.Data); err == nil {
			ele.doc = doc
		}
	} else {
		ele.val.Data = raw.Data
	}
	if err == nil {
		ele.val.Type = raw.Type
	}
	return
}

func (ele *Element) Get(key string) *Element {
	if IsTop(key) {
		return ele
	}
	if ele.val.Type == bsontype.Array {
		return ele.arr.Get(key)
	} else if ele.val.Type == bsontype.EmbeddedDocument {
		return ele.doc.Get(key)
	} else {
		return nil
	}
}

func (ele *Element) Set(i interface{}) error {
	return ele.Marshal(i)
}

func (ele *Element) Pop() (interface{}, error) {
	if ele.val.Type != bsontype.Array {
		return nil, ErrorElementNotSlice
	}
	return ele.arr.Pop()
}

// Push i放入数组，Element必须为Array
func (ele *Element) Push(i interface{}) error {
	if ele.val.Type != bsontype.Array {
		return ErrorElementNotSlice
	}
	return ele.arr.Push(i)
}

func (ele *Element) Unset(key string) error {
	if ele.val.Type != bsontype.EmbeddedDocument {
		return ErrorElementNotDocument
	}
	if ele.doc != nil {
		return ele.doc.Unset(key)
	}
	return nil
}

func (ele *Element) Merge(src *Element, replace bool) {
	if ele.val.Type != src.val.Type {
		if replace {
			ele.val, ele.arr, ele.doc = src.val, src.arr, src.doc
		}
		return
	}
	if ele.val.Type == bsontype.Array {
		ele.arr.Merge(src.arr, replace)
	} else if ele.val.Type == bsontype.EmbeddedDocument {
		ele.doc.Merge(src.doc, replace)
	} else if replace {
		ele.val = src.val
	}
}

// Value 仅当Element为EmbeddedDocument时才可以使用 key
func (ele *Element) Value() bsoncore.Value {
	return ele.build()
}
func (ele *Element) GetBool() (r bool) {
	v := ele.Value()
	r, _ = v.BooleanOK()
	return
}

func (ele *Element) GetInt32() (r int32) {
	v := ele.Value()
	r, _ = v.AsInt32OK()
	return
}

func (ele *Element) GetInt64() (r int64) {
	v := ele.Value()
	r, _ = v.AsInt64OK()
	return
}

func (ele *Element) GetFloat() (r float64) {
	v := ele.Value()
	r, _ = v.AsFloat64OK()
	return
}

func (ele *Element) GetString() (r string) {
	v := ele.Value()
	r, _ = v.StringValueOK()
	return
}

func (ele *Element) String() string {
	raw := ele.Bytes(nil)
	return bsoncore.Element(raw).String()
}

// Bytes 返回 bsoncore.Element 形式的二进制
func (ele *Element) Bytes(dst []byte) []byte {
	if dst == nil {
		dst = make([]byte, 0, ele.Len())
	}
	t := ele.val.Type
	if t == 0 {
		t = bsontype.Null
	}
	dst = bsoncore.AppendHeader(dst, t, ele.key)
	if ele.val.Type == bsontype.Array {
		dst = ele.arr.Bytes(dst)
	} else if ele.val.Type == bsontype.EmbeddedDocument {
		dst = ele.doc.Bytes(dst)
	} else {
		dst = append(dst, ele.val.Data...)
	}

	return dst
}

func (ele *Element) Marshal(i interface{}) error {
	t, b, err := bson.MarshalValue(i)
	if err != nil {
		return err
	}
	if ele.val.Type == bsontype.Array && t == bsontype.Array {
		err = ele.arr.Reset(b)
	} else if ele.val.Type == bsontype.EmbeddedDocument && t == bsontype.EmbeddedDocument {
		err = ele.doc.Reset(b)
	} else {
		ele.val.Type, ele.val.Data = t, b
	}
	return err
}

func (ele *Element) Unmarshal(i interface{}) (err error) {
	if ele.val.Type == bsontype.Array {
		err = ele.arr.Unmarshal(i)
	} else if ele.val.Type == bsontype.EmbeddedDocument {
		err = ele.doc.Unmarshal(i)
	} else {
		raw := bson.RawValue{Value: ele.val.Data, Type: ele.val.Type}
		err = raw.Unmarshal(ele.val)
	}
	return
}

func (ele *Element) build() bsoncore.Value {
	if ele.val.Type == bsontype.Array {
		return ele.arr.build()
	} else if ele.val.Type == bsontype.EmbeddedDocument {
		return ele.doc.build()
	} else {
		return ele.val
	}
}

func (ele *Element) loadOrCreate(key string) (r *Element, loaded bool) {
	if IsTop(key) {
		return ele, true
	}
	if ele.val.Type == 0 {
		ele.val.Type = bsontype.EmbeddedDocument
		ele.doc = New()
	}
	if ele.val.Type == bsontype.Array {
		return ele.arr.loadOrCreate(key)
	} else if ele.val.Type == bsontype.EmbeddedDocument {
		return ele.doc.loadOrCreate(key)
	} else {
		return
	}
}
