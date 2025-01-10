package bson

import (
	"go.mongodb.org/mongo-driver/bson"
)

const PrimaryKey = "_id"

type Collection map[string]Document

func (coll Collection) Has(id string) (r bool) {
	_, r = coll[id]
	return
}
func (coll Collection) Get(id string) (r Document) {
	r, _ = coll[id]
	return
}
func (coll Collection) Len() int {
	return len(coll)
}

// Set 对象插入到集合中，如果已存在则按字段深度匹配覆盖
// i bson二进制或者可以bson.Marshal的 struct、map
func (coll Collection) Set(i interface{}, replace bool) (Document, error) {
	doc, err := Marshal(i)
	if err != nil {
		return nil, err
	}
	id := doc.GetString(PrimaryKey)
	if id == "" {
		return nil, ErrorNoPrimaryKey
	}
	r, ok := coll[id]
	if !ok {
		r = doc
		coll[id] = doc
	} else {
		r.Merge(doc, replace)
	}
	return r, nil
}

func (coll Collection) Count() int {
	return len(coll)
}

func (coll Collection) Range(f func(string, Document) bool) {
	for k, v := range coll {
		if !f(k, v) {
			return
		}
	}
}

// Insert 插入对象,如果已经存在则报错
func (coll Collection) Insert(v interface{}) (doc Document, err error) {
	if doc, err = Marshal(v); err != nil {
		return
	}
	ele := doc.Get(PrimaryKey)
	if ele == nil {
		return nil, ErrorNoPrimaryKey
	}
	oid := ele.GetString()
	if _, ok := coll[oid]; ok {
		return nil, ErrorDocumentExist
	}
	coll[oid] = doc
	return
}

// Delete 删除
func (coll Collection) Delete(id ...string) {
	for _, k := range id {
		delete(coll, k)
	}
}

// Update 更新 TODO
// $set &inc $unset $push $rename
// $setOnInsert
// upsert：true
func (coll Collection) Update(id string, update bson.M) error {
	doc := coll[id]
	if doc == nil {
		return coll.setOnInsert(id, update)
	}
	for k, v := range update {
		if err := updateHandler(doc, k, v); err != nil {
			return err
		}
	}
	return nil
}

// Marshal 将i解析到Document、Element
func (coll Collection) Marshal(id string, i interface{}) (err error) {
	if doc, ok := coll[id]; ok {
		err = doc.Marshal(i)
	}
	return
}

// Unmarshal 使用i解析id对应的文档
func (coll Collection) Unmarshal(id string, i interface{}) (err error) {
	if doc, ok := coll[id]; ok {
		err = doc.Unmarshal(i)
	}
	return
}

func (coll Collection) LoadAndDelete(id string) (value Document, loaded bool) {
	if value, loaded = coll[id]; loaded {
		delete(coll, id)
	}
	return
}

func (coll Collection) setOnInsert(id string, update bson.M) error {
	if _, ok := update[FieldUpdateSetOnInsert]; !ok {
		return ErrorDocumentNotExist
	}
	val := bson.M{}
	for _, k := range fieldUpdateSetOnInsert {
		if v, ok := update[k]; ok {
			mergeSetOnInsertOperators(val, v)
		}
	}
	val[PrimaryKey] = id
	doc, err := Marshal(val)
	if err == nil {
		coll[id] = doc
	}
	return err
}

func mergeSetOnInsertOperators(dist bson.M, src interface{}) {
	m, ok := src.(bson.M)
	if !ok {
		return
	}
	for k, v := range m {
		dist[k] = v
	}
}

func updateHandler(doc Document, k string, v interface{}) error {
	f := fieldUpdateHandler[k]
	if f == nil {
		return nil
	}
	var r bson.M
	switch x := v.(type) {
	case bson.M:
		r = x
	case map[string]interface{}:
		r = x
	default:
		return nil
	}
	return f(doc, r)
}
