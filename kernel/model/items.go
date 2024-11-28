package model

import (
	"errors"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
	"strings"
)

func init() {
	Register(&Items{})
}

type Items struct {
	Model  `bson:"inline"`
	Value  int64         `bson:"val" json:"val"`
	Attach values.Values `bson:"att" json:"att"` //通用字段
}

func (this *Items) Get(k string) (any, bool) {
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

func (this *Items) Set(k string, v any) (any, bool) {
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

func (this *Items) marshal(k string, v any) any {
	if r, err := this.Attach.Marshal(k, v); err != nil {
		logger.Error(err)
		return dataset.Update{} //返回空Update不会向数据库写入错误数据
	} else {
		return r
	}
}

func (this *Items) Copy() *Items {
	i := this.Clone()
	return i.(*Items)
}

// Clone 复制对象,可以将NEW新对象与SET操作解绑
func (this *Items) Clone() any {
	r := *this
	r.Attach = this.Attach.Clone()
	return &r
}

func (this *Items) Upsert(u *updater.Updater, op *operator.Operator) bool {
	return false
}

func (this *Items) Getter(u *updater.Updater, coll *dataset.Collection, keys []string) error {
	uid := u.Uid()
	if uid == 0 {
		return errors.New("Items.Getter uid not found")
	}
	var rows []*Items
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
func (this *Items) Setter(u *updater.Updater, bw dataset.BulkWrite) error {
	return bw.Save()
}

func (this *Items) BulkWrite(u *updater.Updater) dataset.BulkWrite {
	bw := NewBulkWrite(this)
	return bw
}

// TableName 数据库表名
func (*Items) TableName() string {
	return "items"
}

// TableOrder 初始化时的排序，DESC
func (*Items) TableOrder() int32 {
	return 99
}
