package model

import (
	"errors"
	"github.com/hwcer/cosmo/update"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
	"github.com/hwcer/yyds/kernel/config"
	"time"
)

func init() {
	Register(&Shop{})
}

// Shop 商店信息
type Shop struct {
	Model  `bson:"inline"` //iid 对应格子ID
	Val    int32           `json:"val" bson:"val"`       //已经购买数量,刷新会重置此数据
	Goods  int32           `json:"goods" bson:"goods"`   //货物ID
	Expire int64           `json:"expire" bson:"expire"` //过期时间
}

func (this *Shop) Get(k string) (any, bool) {
	switch k {
	case "Val", "val":
		return this.Val, true
	case "Goods", "goods":
		return this.Goods, true
	case "Expire", "expire":
		return this.Expire, true
	default:
		return this.Model.Get(k)
	}
}

// Set 更新器
func (this *Shop) Set(k string, v any) (any, bool) {
	switch k {
	case "Val", "val":
		this.Val = dataset.ParseInt32(v)
	case "Goods", "goods":
		this.Goods = dataset.ParseInt32(v)
	case "Expire", "expire":
		this.Expire = v.(int64)
	default:
		return this.Model.Set(k, v)
	}
	return v, true
}

func (this *Shop) Clone() *Shop {
	r := *this
	return &r
}

func (this *Shop) IType(int32) int32 {
	return config.ITypeShop
}

// ----------------- 作为MODEL方法--------------------

func (this *Shop) Upsert(u *updater.Updater, op *operator.Operator) bool {
	return true
}

func (this *Shop) Getter(u *updater.Updater, coll *dataset.Collection, keys []string) error {
	uid, _ := u.Uid().(uint64)
	if uid == 0 {
		return errors.New("Shop.Getter uid not found")
	}
	tx := DB.Where("uid = ?", uid)
	if len(keys) > 0 {
		tx = tx.Where("_id IN ?", keys)
	}
	var rows []*Shop
	if tx = tx.Find(&rows); tx.Error != nil {
		return tx.Error
	}
	for _, v := range rows {
		coll.Receive(v.OID, v)
	}
	return nil
}

func (this *Shop) Setter(u *updater.Updater, bulkWrite dataset.BulkWrite) error {
	return bulkWrite.Save()
}

func (this *Shop) BulkWrite(u *updater.Updater) dataset.BulkWrite {
	return NewBulkWrite(this)
}

func (this *Shop) BulkWriteFilter(up update.Update) {
	if !up.Has(update.UpdateTypeSet, "update") {
		this.Update = time.Now().Unix()
		up.Set("update", this.Update)
	}
}
