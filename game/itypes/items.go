package model

import (
	"errors"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
	"server/config"
	"server/define"
	"strings"
)

var ITypeItems = newItemsIType(define.ITypeItems)
var ITypeViper = newItemsIType(define.ITypeViper)

func init() {
	im := &Item{}
	Register(im)
	types := []updater.IType{ITypeItems, ITypeViper, ITypeGacha, ITypeTicket}
	types = append(types, ITypeEquip, ITypeHero)
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeAlways, im, types...); err != nil {
		logger.Panic(err)
	}
}

type Item struct {
	Model  `bson:"inline"`
	Value  int64         `bson:"val" json:"val"`
	Attach values.Values `bson:"att" json:"att"` //通用字段
}

func (this *Item) Get(k string) (any, bool) {
	if i := strings.Index(k, "."); i > 0 && k[0:i] == "att" {
		return this.Attach.Get(k[i+1:]), true
	}
	switch k {
	case "Value", "val":
		return this.Value, true
	case "Attach", "att":
		return this.Attach, true
	default:
		return this.Model.Get(k)
	}
}

func (this *Item) Set(k string, v any) (any, bool) {
	if i := strings.Index(k, "."); i > 0 && k[0:i] == "att" {
		return this.marshal(k[i+1:], v), true
	}
	switch k {
	case "Value", "val":
		this.Value = dataset.ParseInt64(v)
	case "Attach", "att":
		this.Attach = v.(values.Values)
	default:
		return this.Model.Set(k, v)
	}
	return v, true
}

func (this *Item) marshal(k string, v any) any {
	if r, err := this.Attach.Marshal(k, v); err != nil {
		logger.Error(err)
		return dataset.Update{} //返回空Update不会向数据库写入错误数据
	} else {
		return r
	}
}

func (this *Item) Copy() *Item {
	i := this.Clone()
	return i.(*Item)
}

// Clone 复制对象,可以将NEW新对象与SET操作解绑
func (this *Item) Clone() any {
	r := *this
	r.Attach = this.Attach.Clone()
	return &r
}

func (this *Item) IType(iid int32) int32 {
	return config.Data.GetIType(iid)
}

func (this *Item) Upsert(u *updater.Updater, op *operator.Operator) bool {
	return false
}

func (this *Item) Getter(u *updater.Updater, coll *dataset.Collection, keys []string) error {
	uid := GetUid(u)
	if uid == 0 {
		return errors.New("Item.Getter uid not found")
	}
	var rows []*Item
	tx := DB.Model(this).Where("uid = ?", uid)
	tx = tx.Omit("uid", "update")
	if len(keys) > 0 {
		tx = tx.Where("_id IN ?", keys)
	}
	if tx = tx.Find(&rows); tx.Error != nil {
		return tx.Error
	} else {
		for _, v := range rows {
			coll.Receive(v.OID, v)
		}
	}
	return nil
}
func (this *Item) Setter(u *updater.Updater, bw dataset.BulkWrite) error {
	return bw.Save()
}

func (this *Item) BulkWrite(u *updater.Updater) dataset.BulkWrite {
	bw := NewBulkWrite(this)
	return bw
}

// TableName 数据库表名
func (*Item) TableName() string {
	return "items"
}

// TableOrder 初始化时的排序，DESC
func (*Item) TableOrder() int32 {
	return 99
}
