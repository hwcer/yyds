package bson

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"strings"
)

type Document map[string]*Element

func (doc Document) Reset(val []byte) error {
	raw := bsoncore.Document(val)
	if err := raw.Validate(); err != nil {
		return err
	}
	arr, err := raw.Elements()
	if err != nil {
		return err
	}
	for _, ele := range arr {
		k := ele.Key()
		v := NewElement(k)
		doc[k] = v
		if err = v.Reset(ele.Value()); err != nil {
			return err
		}
	}
	return nil
}
func (doc Document) Len() (r int) {
	r += 5 //size(int32) + 0x00
	for _, ele := range doc {
		r += ele.Len()
	}

	return
}

// Merge 将doc中的Element合并(覆盖)到当前文档
// replace 强制覆盖,不存在，类型不一样时强制覆盖
func (doc Document) Merge(src Document, replace bool) {
	for k, v := range src {
		if ele, ok := doc[k]; ok {
			ele.Merge(v, replace)
		} else {
			doc[k] = v
		}
	}
}

func (doc Document) Keys() (r []string) {
	for _, e := range doc {
		r = append(r, e.key)
	}
	return
}
func (doc Document) Has(key string) bool {
	if _, ok := doc[key]; ok {
		return true
	}
	return false
}

// Get Element别名
func (doc Document) Get(key string) (r *Element) {
	k1, k2 := Split(key)
	r = doc[k1]
	if r != nil && k2 != "" {
		r = r.Get(k2)
	}
	return
}

func (doc Document) Set(key string, i interface{}) error {
	ele, _ := doc.loadOrCreate(key)
	return ele.Set(i)
}

// Min 如果小于原值就替换，否则就返回一个错误
// 使用 ErrorNotChange 来判断是逻辑错误还是数据无改变
func (doc Document) Min(key string, val interface{}) (r interface{}, err error) {
	if !IsNumber(val) {
		return 0, ErrorNotValidNumber
	}
	ele, _ := doc.loadOrCreate(key)
	return ele.Min(val)
}

// Max 如果大于原值就替换，否则就返回一个错误
// 使用 ErrorNotChange 来判断是逻辑错误还是数据无改变
func (doc Document) Max(key string, val interface{}) (r interface{}, err error) {
	if !IsNumber(val) {
		return 0, ErrorNotValidNumber
	}
	ele, _ := doc.loadOrCreate(key)
	return ele.Max(val)
}

// Inc 自增，类型必须是数字
func (doc Document) Inc(key string, val interface{}) (r interface{}, err error) {
	if !IsNumber(val) {
		return 0, ErrorNotValidNumber
	}
	ele, _ := doc.loadOrCreate(key)
	return ele.Inc(val)
}

func (doc Document) Mul(key string, val interface{}) (r interface{}, err error) {
	if !IsNumber(val) {
		return 0, ErrorNotValidNumber
	}
	ele, _ := doc.loadOrCreate(key)
	return ele.Mul(val)
}

// Pop 删除并返回
// r==nil 时数组不存在或者为空
func (doc Document) Pop(key string) (r interface{}, err error) {
	if ele := doc.Get(key); ele != nil {
		return ele.Pop()
	}
	return
}

// Push appends a specified value to an array
func (doc Document) Push(key string, val interface{}) (err error) {
	if ele := doc.Get(key); ele != nil {
		err = ele.Push(val)
	}
	return
}

// Unset 删除key
func (doc Document) Unset(key string) (err error) {
	i := strings.LastIndex(key, ".")
	if i < 0 {
		delete(doc, key)
		return nil
	}
	ele := doc.Get(key[0:i])
	if ele == nil {
		err = ele.Unset(key[i+1:])
	}

	return
}

func (doc Document) Value(key string) bsoncore.Value {
	if IsTop(key) {
		return bsoncore.Value{Data: doc.Bytes(nil), Type: bsontype.EmbeddedDocument}
	}
	ele := doc.Get(key)
	if ele != nil {
		return ele.Value()
	}
	return bsoncore.Value{}
}
func (doc Document) GetBool(key string) (r bool) {
	if ele := doc.Get(key); ele != nil {
		r = ele.GetBool()
	}
	return
}

func (doc Document) GetInt32(key string) (r int32) {
	if ele := doc.Get(key); ele != nil {
		r = ele.GetInt32()
	}
	return
}

func (doc Document) GetInt64(key string) (r int64) {
	if ele := doc.Get(key); ele != nil {
		r = ele.GetInt64()
	}
	return
}

func (doc Document) GetFloat(key string) (r float64) {
	if ele := doc.Get(key); ele != nil {
		r = ele.GetFloat()
	}
	return
}

func (doc Document) GetString(key string) (r string) {
	if ele := doc.Get(key); ele != nil {
		r = ele.GetString()
	}
	return
}

func (doc Document) Bytes(dst []byte) []byte {
	if dst == nil {
		dst = make([]byte, 0, doc.Len())
	}
	idx, dst := bsoncore.ReserveLength(dst)
	for _, e := range doc {
		dst = e.Bytes(dst)
	}
	dst = append(dst, 0x00)
	dst = bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
	return dst
}

func (doc Document) String() string {
	raw := doc.Bytes(nil)
	return bsoncore.Document(raw).String()
}

func (doc Document) Marshal(i interface{}) (err error) {
	t, b, err := bson.MarshalValue(i)
	if err != nil {
		return
	}
	if t != bsontype.EmbeddedDocument {
		return ErrorElementNotDocument
	}
	return doc.Reset(b)
}

func (doc Document) Unmarshal(i interface{}) error {
	raw := doc.Bytes(nil)
	return bson.Unmarshal(raw, i)
}

func (doc Document) build() bsoncore.Value {
	return bsoncore.Value{Data: doc.Bytes(nil), Type: bsontype.EmbeddedDocument}
}

func (doc Document) loadOrCreate(key string) (r *Element, loaded bool) {
	k1, k2 := Split(key)
	if r = doc[k1]; r == nil {
		r = NewElement(k1)
		doc[k1] = r
	} else {
		loaded = true
	}
	if k2 != "" {
		if loaded {
			r, loaded = r.loadOrCreate(k2)
		} else {
			r, _ = r.loadOrCreate(k2)
		}

	}
	return
}
