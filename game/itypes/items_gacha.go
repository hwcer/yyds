package itypes

import (
	"server/define"
)

var ITypeGacha = newItemsIType(define.ITypeGacha)

func init() {
	ITypeGacha.SetStacked(true)
	//ITypeGacha.SetAttach(itemsEquipAttach)

	//im := &Gacha{}
	//Register(im)
	//if err := updater.Register(updater.ParserTypeCollection, updater.RAMTypeNone, im, ITypeGacha); err != nil {
	//	logger.Panic(err)
	//}
}

const (
	GachaAttachLess = "less" //累计出现保底消耗的次数
	GachaAttachSpec = "spec" //累计出现保底次数
	GachaAttachWish = "wish" //许愿池 GachaRate -> GachaGroup  map[int32]int32
)

// var ITypeGacha = &gachaIType{NewIType(define.ITypeGacha)}
//
//// Gacha 扭蛋机
//type Gacha struct {
//	Model `bson:"inline"`
//	Val   int32           `json:"val" bson:"val"`   //累计抽卡次数
//	Less  int32           `json:"less" bson:"less"` //累计出现保底消耗的次数
//	Spec  int32           `json:"spec" bson:"spec"` //累计出现保底次数
//	Wish  map[int32]int32 `json:"wish" bson:"wish"` //许愿池 GachaRate -> GachaGroup
//}
//
//func (this *Gacha) Get(k string) (any, bool) {
//	switch k {
//	case "Val", "val":
//		return this.Val, true
//	case "Less", "less":
//		return this.Less, true
//	case "Spec", "spec":
//		return this.Spec, true
//	default:
//		return this.Model.Get(k)
//	}
//}
//
//// Set 更新器
//func (this *Gacha) Set(k string, v any) (any, bool) {
//	switch k {
//	case "Val", "val":
//		this.Val = dataset.ParseInt32(v)
//	case "Less", "less":
//		this.Less = dataset.ParseInt32(v)
//	case "Spec", "spec":
//		this.Spec = dataset.ParseInt32(v)
//	default:
//		return this.Model.Set(k, v)
//	}
//	return v, true
//}
//
//func (this *Gacha) Clone() any {
//	s := *this
//	r := &s
//	r.Wish = make(map[int32]int32, len(this.Wish))
//	for k, v := range this.Wish {
//		r.Wish[k] = v
//	}
//	return r
//}
//func (this *Gacha) Saving(u dataset.Update) {
//	if _, ok := u["update"]; !ok {
//		u["update"] = time.Now()
//	}
//}
//
//func (this *Gacha) IType(int32) int32 {
//	return define.ITypeGacha
//}
//
//// ----------------- 作为MODEL方法--------------------
//
//func (this *Gacha) Upsert(u *updater.Updater, op *operator.Operator) bool {
//	return true
//}
//
//func (this *Gacha) Getter(u *updater.Updater, coll *dataset.Collection, keys []string) error {
//	//if len(keys) == 0 {
//	//	return errors.New("Gacha.Getter keys empty")
//	//}
//	uid := Uid(u)
//	if uid == 0 {
//		return errors.New("Gacha.Getter uid not found")
//	}
//	tx := DB.Where("uid = ?", uid)
//	if len(keys) > 0 {
//		tx = tx.Where("_id IN ?", keys)
//	}
//
//	tx = tx.Omit("uid", "update")
//	var rows []*Gacha
//	if tx = tx.Find(&rows); tx.Error != nil {
//		return tx.Error
//	}
//	for _, v := range rows {
//		coll.Receive(v.OID, v)
//	}
//	return nil
//}
//func (this *Gacha) Setter(u *updater.Updater, bulkWrite dataset.BulkWrite) error {
//	return bulkWrite.(*cosmo.BulkWrite).Save()
//}
//func (this *Gacha) BulkWrite(u *updater.Updater) dataset.BulkWrite {
//	return DB.BulkWrite(this)
//}
//
//type gachaIType struct {
//	*IType
//}
//
//func (this *gachaIType) New(u *updater.Updater, op *operator.Operator) (any, error) {
//	return this.Create(u, op.IID, op.Value), nil
//}
//func (this *gachaIType) Stacked() bool {
//	return true
//}
//func (this *gachaIType) ObjectId(u *updater.Updater, iid int32) (string, error) {
//	return Unique(u, iid)
//}
//func (this *gachaIType) Create(u *updater.Updater, iid int32, val int64) *Gacha {
//	i := &Gacha{}
//	i.Init(u, iid)
//	i.OID, _ = this.ObjectId(u, iid)
//	i.Val = int32(val)
//	return i
//}
