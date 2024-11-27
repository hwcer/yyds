package model

import (
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo/update"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
	"server/config"
	"server/define"
	"strconv"
	"strings"
	"time"
)

var ITypeActive = NewActiveIType(define.ITypeActive)

func init() {
	im := &Active{}
	Register(im)
	var its []updater.IType
	its = append(its, ITypeActive)
	its = append(its, NewActiveIType(define.ITypeConfig))
	if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeAlways, im, its...); err != nil {
		logger.Panic(err)
	}
}

// Active 运营活动
type Active struct {
	Model  `bson:"inline"`
	Attach values.Values `json:"att" bson:"att"` //数据
	Expire int64         `json:"ttl" bson:"ttl"` //过期时间
}

func (this *Active) Get(k string) (any, bool) {
	if i := strings.Index(k, "."); i > 0 && k[0:i] == "att" {
		return this.Attach.Get(k[i+1:]), true
	}
	switch k {
	case "Attach", "att":
		return this.Attach, true
	case "Expire", "ttl":
		return this.Expire, true
	default:
		return this.Model.Get(k)
	}
}

// Set 更新器
func (this *Active) Set(k string, v any) (any, bool) {
	if i := strings.Index(k, "."); i > 0 && k[0:i] == "att" {
		return this.marshal(k[i+1:], v), true
	}
	switch k {
	case "Attach", "att":
		this.Attach = v.(values.Values)
	case "Expire", "ttl":
		this.Expire = v.(int64)
	default:
		return this.Model.Set(k, v)
	}
	return v, true
}
func (this *Active) marshal(k string, v any) any {
	if r, err := this.Attach.Marshal(k, v); err != nil {
		logger.Error(err)
		return dataset.Update{} //返回空Update不会向数据库写入错误数据
	} else {
		return r
	}
}
func (this *Active) IType(id int32) int32 {
	s := strconv.Itoa(int(id))
	if len(s) < 2 {
		return 0
	}
	i, _ := strconv.Atoi(s[0:2])
	return int32(i)
}
func (this *Active) Copy() *Active {
	i := this.Clone()
	return i.(*Active)
}

// ----------------- 作为MODEL方法--------------------

func (this *Active) Clone() any {
	r := *this
	r.Attach = this.Attach.Clone()
	return &r
}

func (this *Active) Upsert(u *updater.Updater, op *operator.Operator) bool {
	return true
}

func (this *Active) Getter(u *updater.Updater, coll *dataset.Collection, keys []string) error {
	tx := DB.Omit("uid", "update")
	var ids []string
	if len(keys) == 0 {
		for k, _ := range config.Data.Active {
			o, _ := Unique(u, k)
			ids = append(ids, o)
		}
	} else {
		ids = keys
	}

	tx = tx.Where("_id IN ?", ids)
	//tx := DB.Where("uid = ?", uid)

	var rows []*Active
	if tx = tx.Find(&rows); tx.Error != nil {
		return tx.Error
	}
	for _, v := range rows {
		coll.Receive(v.OID, v)
	}
	return nil
}
func (this *Active) Setter(u *updater.Updater, bw dataset.BulkWrite) error {
	return bw.Save()
}
func (this *Active) BulkWrite(u *updater.Updater) dataset.BulkWrite {
	bw := NewBulkWrite(this)
	return bw
}
func (this *Active) BulkWriteFilter(up update.Update) {
	if !up.Has(update.UpdateTypeSet, "update") {
		this.Update = time.Now().Unix()
		up.Set("update", this.Update)
	}
}

type ActiveIType struct {
	IType
}

func NewActiveIType(id int32) *ActiveIType {
	it := &ActiveIType{}
	it.IType = *NewIType(id)
	it.IType.SetStacked(true)
	it.IType.SetCreator(it.creator)
	return it
}
func (this *ActiveIType) creator(u *updater.Updater, iid int32, val int64) (any, error) {
	var err error
	i := &Active{}
	i.Model.Init(u, iid)
	i.OID, err = this.ObjectId(u, iid)
	i.Update = u.Time.Unix()
	return i, err
}
func (this *ActiveIType) Create(u *updater.Updater, iid int32, val int64) (*Active, error) {
	if i, err := this.creator(u, iid, val); err == nil {
		return i.(*Active), nil
	} else {
		return nil, err
	}
}
