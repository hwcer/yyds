package player

import (
	"fmt"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"server/define"
	"server/game/model"
)

func init() {
	Register("itemsIndexesWorker", itemsIndexesWorker)
}

type itemsIndexesMonitor struct {
	item *Items
}

func (this *itemsIndexesMonitor) Insert(doc *dataset.Document) {
	this.item.addIndexes(doc)
}
func (this *itemsIndexesMonitor) Delete(doc *dataset.Document) {
	this.item.delIndexes(doc)
}

type itemsIndexes map[string]*dataset.Document

// itemsIndexesWorker 整理items索引
func itemsIndexesWorker(p *Player) any {
	if !p.Items.Collection.Loader() {
		return nil
	}
	p.Items.Collection.SetMonitor(&itemsIndexesMonitor{item: p.Items})
	p.Items.Collection.Range(func(id string, doc *dataset.Document) bool {
		p.Items.addIndexes(doc)
		return true
	})
	return nil
}

func (this *Items) delIndexes(doc *dataset.Document) {
	item, ok := doc.Any().(*model.Item)
	if !ok {
		return
	}
	it := item.IType(item.IID)
	if index := this.indexes[it]; index != nil {
		delete(index, item.OID)
	}
}
func (this *Items) addIndexes(doc *dataset.Document) {
	item, ok := doc.Any().(*model.Item)
	if !ok {
		return
	}
	it := item.IType(item.IID)
	index := this.indexes[it]
	if index == nil {
		index = itemsIndexes{}
		this.indexes[it] = index
	}
	index[item.OID] = doc
}

func NewItems(p *Player) *Items {
	doc := p.Collection(define.ITypeItems)
	r := &Items{Collection: doc, player: p}
	r.indexes = make(map[int32]itemsIndexes)
	return r
}

type Items struct {
	*updater.Collection
	player  *Player
	indexes map[int32]itemsIndexes
}

// Get 总是返回TASK对象
func (this *Items) Get(id any) (r *model.Item) {
	if v := this.Collection.Get(id); v == nil {
		return
	} else {
		r = v.(*model.Item)
	}
	return
}
func (this *Items) GetOrCreate(id int32, autoInsertDB bool) (r *model.Item, exist bool) {
	if i := this.Collection.Get(id); i != nil {
		exist = true
		r = i.(*model.Item)
	} else {
		r, _ = model.ITypeItems.Create(this.player.Updater, id, 0)
		if autoInsertDB {
			_ = this.Collection.New(r)
		}
	}
	return
}

func (this *Items) SetAttach(id any, k string, v any) {
	s := fmt.Sprintf("att.%d", k)
	this.Collection.Set(id, s, v)
}

func (this *Items) GetAttach(id any, k string) any {
	d := this.Get(id)
	if d == nil {
		return nil
	}
	return d.Attach.Get(k)
}

func (this *Items) Range(h func(id string, active *model.Item) bool) {
	this.Collection.Range(func(id string, doc *dataset.Document) bool {
		v, _ := doc.Any().(*model.Item)
		return h(id, v)
	})
}

// Count 按IType统计记录数，不是道具数量
func (this *Items) Count(it ...int32) (r int) {
	for _, i := range it {
		r += len(this.indexes[i])
	}
	return
}

func (this *Items) RangeWithIType(it int32, h func(id string, active *model.Item) bool) {
	for k, doc := range this.indexes[it] {
		v, _ := doc.Any().(*model.Item)
		if !h(k, v) {
			return
		}
	}
}
