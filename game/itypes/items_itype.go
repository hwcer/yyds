package itypes

import (
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/updater"
)

func newItemsIType(id int32) *itemsIType {
	i := &itemsIType{}
	i.IType.id = id
	i.IType.stacked = true
	i.IType.creator = i.creator
	return i
}

type itemsIType struct {
	IType
	attach  func(u *updater.Updater, item *Item) (any, error)    //设置attach
	resolve func(u *updater.Updater, iid int32, val int64) error //分解
}

func (this *itemsIType) SetAttach(attach func(u *updater.Updater, item *Item) (any, error)) {
	this.attach = attach
}
func (this *itemsIType) SetResolve(resolve func(u *updater.Updater, iid int32, val int64) error) {
	this.resolve = resolve
}

// Resolve 自动分解
func (this *itemsIType) Resolve(u *updater.Updater, iid int32, val int64) error {
	if this.resolve != nil {
		return this.resolve(u, iid, val)
	}
	return nil
}

func (this *itemsIType) Create(u *updater.Updater, iid int32, val int64) (*Item, error) {
	if i, err := this.creator(u, iid, val); err != nil {
		return nil, err
	} else {
		return i.(*Item), nil
	}
}

func (this *itemsIType) creator(u *updater.Updater, iid int32, val int64) (any, error) {
	r := &Item{}
	r.Init(u, iid)
	var err error
	if r.OID, err = this.IType.ObjectId(u, iid); err != nil {
		return nil, err
	}
	r.Value = val
	r.Attach = values.Values{}
	if this.attach != nil {
		return this.attach(u, r)
	}
	return r, nil
}
