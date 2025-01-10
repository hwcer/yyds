package dataset

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/values"
)

const (
	collOperatorInsert int = 1
	collOperatorUpdate int = 2
	collOperatorDelete int = 3
)

type Operator struct {
	op  values.Byte
	doc *Document
}

type Dirty map[string]*Operator

func (c Dirty) Has(k string) (ok, exist bool) {
	v, ok := c[k]
	if !ok {
		return false, false
	}
	if v.op.Has(collOperatorInsert) {
		return true, true
	} else if v.op.Has(collOperatorDelete) {
		return false, true
	}
	return false, false
}

func (c Dirty) Get(k string) *Document {
	if v, ok := c[k]; ok && v.op.Has(collOperatorInsert) {
		return v.doc
	}
	return nil
}

func (c Dirty) Remove(k string) {
	delete(c, k)
}

// Delete 标记为删除
func (c Dirty) Delete(k string) {
	d := c.Operator(k)
	d.op = 0
	d.doc = nil
	d.op.Set(collOperatorDelete)
}

// Update 标记为更新
func (c Dirty) Update(k string) {
	d := c.Operator(k)
	if d.op.Has(collOperatorDelete) && !d.op.Has(collOperatorInsert) {
		logger.Alert("已经标记为删除的记录无法直接再次使用Update操作:%v", k)
		return
	}
	d.op.Set(collOperatorUpdate)
}

// Insert 临时缓存新对象
func (c Dirty) Insert(k string, doc *Document) {
	d := c.Operator(k)
	d.doc = doc
	d.op.Set(collOperatorInsert)
	d.op.Delete(collOperatorUpdate) //Insert取消Update操作
}

//func (c Dirty) Release() {
//	for _, v := range c {
//		if v.op.Has(collOperatorInsert) {
//			v.doc.Release()
//		}
//	}
//}

func (c Dirty) Operator(k string) (r *Operator) {
	r, ok := c[k]
	if !ok {
		r = &Operator{}
		c[k] = r
	}
	return
}
