package dataset

import (
	"fmt"
)

type CollectionMonitor interface {
	Insert(doc *Document)
	Delete(doc *Document)
}

func NewColl(rows ...any) *Collection {
	coll := &Collection{}
	coll.dataset = Dataset{}
	coll.Reset(rows...)
	return coll
}

type Dataset map[string]*Document

func (d Dataset) Set(k string, doc *Document) {
	d[k] = doc
}
func (d Dataset) Has(k string) (ok bool) {
	_, ok = d[k]
	return
}
func (d Dataset) Del(k string) {
	delete(d, k)
}

func (d Dataset) Get(k string) (doc *Document, ok bool) {
	doc, ok = d[k]
	return
}

func (d Dataset) GetAndDel(k string) (doc *Document) {
	if doc = d[k]; doc != nil {
		delete(d, k)
	}
	return
}

type Collection struct {
	dirty   Dirty   //临时数据
	dataset Dataset //数据集
}

func (coll *Collection) Len() int {
	return len(coll.dataset)
}

// Has 是否存在记录，包括已经标记为删除记录，主要用来判断是否已经拉取过数据
func (coll *Collection) Has(id string) bool {
	if ok, exist := coll.dirty.Has(id); exist {
		return ok
	} else if coll.dataset.Has(id) {
		return true
	}
	return false
}

// Get 获取对象，已经标记为删除的对象被视为不存在
func (coll *Collection) Get(id string) (*Document, bool) {
	if r := coll.dirty.Get(id); r != nil {
		return r, true
	}
	return coll.dataset.Get(id)
}

func (coll *Collection) Val(id string) (r *Document) {
	r, _ = coll.Get(id)
	return
}

func (coll *Collection) Set(id string, field string, value any) error {
	data := make(map[string]any)
	data[field] = value
	return coll.Update(id, data)
}

func (coll *Collection) New(i ...any) (err error) {
	for _, v := range i {
		if err = coll.Insert(v); err != nil {
			return
		}
	}
	return
}

// Update 批量更新,对象必须已经存在
func (coll *Collection) Update(id string, data Update) error {
	doc, ok := coll.Get(id)
	if !ok {
		return fmt.Errorf("item not exist:%v", id)
	}
	doc.Update(data)
	dirty := coll.Dirty()
	dirty.Update(id)
	return nil
}

// Insert 如果已经存在转换成覆盖
func (coll *Collection) Insert(i any) (err error) {
	doc := NewDoc(i)
	id := doc.GetString(ItemNameOID)
	if id == "" {
		return fmt.Errorf("item id emtpy:%v", i)
	}
	if coll.Has(id) {
		return fmt.Errorf("item already exist:%v", id)
	}
	dirty := coll.Dirty()
	dirty.Insert(id, doc)
	return
}

func (coll *Collection) Delete(id string) {
	dirty := coll.Dirty()
	dirty.Delete(id)
}

// Remove 从内存中清理，不会触发持久化操作
func (coll *Collection) Remove(id ...string) {
	for _, k := range id {
		delete(coll.dirty, k)
		delete(coll.dataset, k)
	}
}

func (coll *Collection) Save(bulkWrite BulkWrite, monitor CollectionMonitor) error {
	for k, v := range coll.dirty {
		if v.op.Has(collOperatorDelete) {
			doc := coll.dataset.GetAndDel(k)
			if bulkWrite != nil {
				bulkWrite.Delete(k)
			}
			if monitor != nil && doc != nil {
				monitor.Delete(doc)
			}
		}
		if v.op.Has(collOperatorInsert) {
			doc := v.doc
			if v.op.Has(collOperatorUpdate) {
				doc = doc.Clone()
			}
			//整合collOperatorUpdate
			if err := doc.Save(nil); err == nil {
				coll.dataset.Set(k, doc)
				if bulkWrite != nil {
					bulkWrite.Insert(doc.Any())
				}
			}
			if monitor != nil {
				monitor.Insert(doc)
			}
		} else if v.op.Has(collOperatorUpdate) {
			doc, _ := coll.dataset.Get(k)
			dirty := make(Update)
			if err := doc.Save(dirty); err == nil && len(dirty) > 0 && bulkWrite != nil {
				bulkWrite.Update(dirty, k)
			}
		}
	}
	coll.dirty = nil
	return nil
}

func (coll *Collection) Range(handle func(string, *Document) bool) {
	for k, v := range coll.dataset {
		if !handle(k, v) {
			return
		}
	}
}

func (coll *Collection) Reset(rows ...any) {
	coll.dataset = make(Dataset, len(rows))
	coll.dirty = nil
	for _, i := range rows {
		_ = coll.create(i)
	}
}

// Receive 接收器，接收外部对象放入列表，不进行任何操作，一般用于初始化
func (coll *Collection) Receive(id string, data any) {
	coll.dataset.Set(id, NewDoc(data))
}
func (coll *Collection) create(i any) (err error) {
	doc := NewDoc(i)
	if id := doc.GetString(ItemNameOID); id != "" {
		coll.dataset.Set(id, doc)
	} else {
		err = fmt.Errorf("item id empty:%+v", i)
	}
	return
}

func (coll *Collection) Dirty() Dirty {
	if coll.dirty == nil {
		coll.dirty = Dirty{}
	}
	return coll.dirty
}
