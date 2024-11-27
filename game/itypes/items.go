package itypes

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/game/model"
	"github.com/hwcer/yyds/game/share"
)

var Items = NewItemsIType(share.ITypeItems)
var Viper = NewItemsIType(share.ITypeViper)

func init() {
	im := &model.Items{}
	its := []updater.IType{Items, Viper, Gacha, Ticket}
	its = append(its, Equip, Hero)
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeAlways, im, its...); err != nil {
		logger.Panic(err)
	}
}

func NewItemsIType(id int32) *ItemsIType {
	i := &ItemsIType{}
	i.IType.id = id
	i.IType.stacked = true
	i.IType.creator = i.creator
	return i
}

type ItemsIType struct {
	IType
	attach  func(u *updater.Updater, item *model.Items) (any, error) //设置attach
	resolve func(u *updater.Updater, iid int32, val int64) error     //分解
}

func (this *ItemsIType) SetAttach(attach func(u *updater.Updater, item *model.Items) (any, error)) {
	this.attach = attach
}

func (this *ItemsIType) SetResolve(resolve func(u *updater.Updater, iid int32, val int64) error) {
	this.resolve = resolve
}

// Resolve 自动分解
func (this *ItemsIType) Resolve(u *updater.Updater, iid int32, val int64) error {
	if this.resolve != nil {
		return this.resolve(u, iid, val)
	}
	return nil
}

func (this *ItemsIType) Create(u *updater.Updater, iid int32, val int64) (*model.Items, error) {
	if i, err := this.creator(u, iid, val); err != nil {
		return nil, err
	} else {
		return i.(*model.Items), nil
	}
}

func (this *ItemsIType) creator(u *updater.Updater, iid int32, val int64) (any, error) {
	r := &model.Items{}
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
