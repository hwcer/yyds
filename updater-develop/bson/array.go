package bson

import (
	"github.com/hwcer/cosgo/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"strconv"
)

type Array struct {
	dict []*Element
}

func (arr *Array) Reset(v []byte) error {
	raw := bsoncore.Array(v)
	if err := raw.Validate(); err != nil {
		return err
	}
	values, err := raw.Values()
	if err != nil {
		return err
	}
	dict := make([]*Element, len(values))
	for i, value := range values {
		k := strconv.Itoa(i)
		ele := NewElement(k)
		if err = ele.Reset(value); err != nil {
			return err
		}
		dict[i] = ele
	}
	arr.dict = dict
	return nil
}

func (arr *Array) Len() (r int) {
	r += 5
	for i, ele := range arr.dict {
		if ele == nil {
			r += len(strconv.Itoa(i)) + 2
		} else {
			r += ele.Len()
		}
	}
	return
}

func (arr *Array) Get(key string) (r *Element) {
	k1, k2 := Split(key)
	idx, err := strconv.Atoi(k1)
	if err == nil && idx < 0 {
		err = ErrorSliceIndexIllegal
	}
	if err != nil {
		logger.Error(err)
		return
	}
	r = arr.dict[idx]
	if r != nil && k2 != "" {
		r = r.Get(k2)
	}
	return
}

func (arr *Array) Set(key string, i interface{}) error {
	ele, _ := arr.loadOrCreate(key)
	return ele.Marshal(i)
}

func (arr *Array) Pop() (r interface{}, err error) {
	l := len(arr.dict)
	if l == 0 {
		return nil, nil
	}
	idx := l - 1
	r = arr.dict[idx]
	arr.dict = arr.dict[0:idx]
	return
}

func (arr *Array) Push(i interface{}) error {
	k := strconv.Itoa(len(arr.dict))
	ele, err := NewElementFromValue(k, i)
	if err != nil {
		return err
	}
	arr.dict = append(arr.dict, ele)
	return nil
}
func (arr *Array) Merge(src *Array, replace bool) {
	if replace {
		arr.dict = src.dict
	} else if len(src.dict) > len(arr.dict) {
		arr.dict = append(arr.dict, src.dict[len(arr.dict):]...)
	}
}

func (arr *Array) Value(key string) bsoncore.Value {
	if IsTop(key) {
		return arr.build()
	}
	ele := arr.Get(key)
	if ele != nil {
		return ele.Value()
	}
	return bsoncore.Value{}
}

func (arr *Array) Bytes(dst []byte) []byte {
	if dst == nil {
		dst = make([]byte, 0, arr.Len())
	}
	idx, dst := bsoncore.ReserveLength(dst)
	for i, ele := range arr.dict {
		if ele == nil {
			dst = bsoncore.AppendNullElement(dst, strconv.Itoa(i))
		} else {
			dst = ele.Bytes(dst)
		}
	}
	dst = append(dst, 0x00)
	dst = bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
	return dst
}

func (arr *Array) build() bsoncore.Value {
	return bsoncore.Value{Data: arr.Bytes(nil), Type: bsontype.Array}
}

func (arr *Array) String() string {
	raw := arr.Bytes(nil)
	return bsoncore.Array(raw).String()
}

// marshal 编译对象
func (arr *Array) Marshal(i interface{}) (err error) {
	t, b, err := bson.MarshalValue(i)
	if err != nil {
		return
	}
	if t != bsontype.Array {
		return ErrorElementNotSlice
	}
	return arr.Reset(b)
}

func (arr *Array) Unmarshal(i interface{}) error {
	raw := arr.Bytes(nil)
	return bson.Unmarshal(raw, i)
}

// element 获取子对象
func (arr *Array) loadOrCreate(key string) (r *Element, loaded bool) {
	k1, k2 := Split(key)
	idx, _ := strconv.Atoi(k1)
	if idx < 0 {
		idx = 0
	}
	//自动扩容
	minLen := idx + 1
	if minLen > len(arr.dict) {
		r = NewElement(k1)
		dict := make([]*Element, minLen)
		copy(dict, arr.dict)
		dict[idx] = r
		arr.dict = dict

	} else {
		r = arr.dict[idx]
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
