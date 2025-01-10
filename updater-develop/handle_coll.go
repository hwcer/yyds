package updater

import (
	"fmt"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
)

type collectionModel interface {
	Upsert(update *Updater, op *operator.Operator) bool
	Getter(update *Updater, data *dataset.Collection, keys []string) error //keys==nil 初始化所有
	Setter(update *Updater, bulkWrite dataset.BulkWrite) error
	BulkWrite(update *Updater) dataset.BulkWrite
}

// collectionUpsert set时如果不存在,是否自动转换为new
//type collectionUpsert interface {
//	Upsert(update *Updater, op *operator.Operator) bool
//}

type Collection struct {
	statement
	model     collectionModel
	remove    []string //需要移除内存的数据,仅仅RAMMaybe有效
	dataset   *dataset.Collection
	monitor   dataset.CollectionMonitor
	bulkWrite dataset.BulkWrite
}

func NewCollection(u *Updater, model any) Handle {
	r := &Collection{}
	r.model = model.(collectionModel)
	r.statement = *newStatement(u, r.operator, r.Has)
	return r
}
func (this *Collection) Parser() Parser {
	return ParserTypeCollection
}

func (this *Collection) SetMonitor(v dataset.CollectionMonitor) {
	this.monitor = v
}

//func (this *Collection) get(k string) (r *dataset.Document) {
//	return this.dataset.Get(k)
//}

func (this *Collection) val(id string) (r int64, ok bool) {
	var i *dataset.Document
	if i, ok = this.dataset.Get(id); ok {
		r = i.GetInt64(dataset.ItemNameVAL)
	}
	return
}

func (this *Collection) save() (err error) {
	if this.Updater.Async || this.dataset == nil {
		return
	}
	bulkWrite := this.BulkWrite()
	if err = this.dataset.Save(bulkWrite, this.monitor); err != nil {
		return
	}
	if err = this.model.Setter(this.statement.Updater, bulkWrite); err == nil {
		this.bulkWrite = nil
	}
	return
}

func (this *Collection) reset() {
	this.statement.reset()
	if this.dataset == nil {
		this.dataset = dataset.NewColl()
	}
}

func (this *Collection) release() {
	this.statement.release()
	if !this.Updater.Async {
		if this.statement.ram == RAMTypeNone {
			this.dataset = nil
		}
	}
}
func (this *Collection) loading(ram RAMType) error {
	if this.dataset == nil {
		this.dataset = dataset.NewColl()
	}
	this.statement.ram = ram
	if !this.statement.loader && (this.statement.ram == RAMTypeMaybe || this.statement.ram == RAMTypeAlways) {
		this.statement.loader = true
		this.Updater.Error = this.model.Getter(this.Updater, this.dataset, nil)
	}
	return this.Updater.Error
}

// 关闭时执行,玩家下线
func (this *Collection) destroy() (err error) {
	return this.save()
}

func (this *Collection) Len() int {
	return this.dataset.Len()
}

func (this *Collection) Has(id any) (r bool) {
	if oid, err := this.ObjectId(id); err == nil {
		r = this.dataset.Has(oid)
	} else {
		logger.Debug(err)
	}
	return
}

// Get 返回item,不可叠加道具只能使用oid获取
func (this *Collection) Get(key any) (r any) {
	if doc := this.Doc(key); doc != nil {
		r = doc.Any()
	}
	return
}

// Val 直接获取 item中的val值,不可叠加道具只能使用oid获取
func (this *Collection) Val(key any) (r int64) {
	if oid, err := this.ObjectId(key); err == nil {
		r, _ = this.val(oid)
	}
	return
}
func (this *Collection) Doc(key any) (r *dataset.Document) {
	if oid, err := this.ObjectId(key); err == nil {
		r = this.dataset.Val(oid)
	} else {
		logger.Debug(err)
	}
	return
}

// Set 设置 k= oid||iid
// Set(oid||iid,map[string]any)
// Set(oid||iid,key string,val any)
func (this *Collection) Set(k any, v ...any) {
	switch len(v) {
	case 1:
		if update := dataset.ParseUpdate(v[0]); update != nil {
			this.operator(operator.TypesSet, k, 0, update)
		} else {
			this.Updater.Error = ErrArgsIllegal(k, v)
		}
	case 2:
		if field, ok := v[0].(string); ok {
			this.operator(operator.TypesSet, k, 0, dataset.NewUpdate(field, v[1]))
		} else {
			this.Updater.Error = ErrArgsIllegal(k, v)
		}
	default:
		this.Updater.Error = ErrArgsIllegal(k, v)
	}
}

// Remove 从内存中移除，用于清理不常用数据，不会改变数据库
func (this *Collection) Remove(id ...string) {
	this.remove = append(this.remove, id...)
}

// New 使用全新的模型插入
func (this *Collection) New(v dataset.Model) (err error) {
	op := &operator.Operator{OID: v.GetOID(), IID: v.GetIID(), Type: operator.TypesNew, Result: []any{v}}
	op.Value = 1
	if err = this.mayChange(op); err != nil {
		return this.Updater.Errorf(err)
	}
	this.statement.Operator(op)
	return
}

func (this *Collection) Select(keys ...any) {
	for _, k := range keys {
		if oid, err := this.ObjectId(k); err == nil {
			this.statement.Select(oid)
		} else {
			logger.Alert(err)
		}
	}
}

func (this *Collection) Data() (err error) {
	if this.Updater.Error != nil {
		return this.Updater.Error
	}
	if len(this.keys) == 0 {
		return nil
	}
	keys := this.keys.ToString()
	if err = this.model.Getter(this.Updater, this.dataset, keys); err == nil {
		this.statement.date()
	}
	return
}

func (this *Collection) verify() (err error) {
	if err = this.Updater.WriteAble(); err != nil {
		return
	}
	for _, act := range this.statement.operator {
		if err = this.Parse(act); err != nil {
			return
		}
	}
	this.statement.verify()
	return
}

func (this *Collection) submit() (err error) {
	if err = this.Updater.WriteAble(); err != nil {
		return
	}
	this.statement.submit()
	if err = this.save(); err != nil && this.ram != RAMTypeNone {
		logger.Alert("同步数据失败,等待下次同步:%v", err)
		err = nil
	}
	if len(this.remove) > 0 {
		this.dataset.Remove(this.remove...)
		this.remove = nil
	}

	return
}

// Len 总记录数
//func (this *Collection) Len() int {
//	return len(this.dataset)
//}

func (this *Collection) Range(h func(id string, doc *dataset.Document) bool) {
	this.dataset.Range(h)
}

func (this *Collection) IType(iid int32) IType {
	if h, ok := this.model.(modelIType); ok {
		v := h.IType(iid)
		return itypesDict[v]
	} else {
		return this.Updater.IType(iid)
	}
}

func (this *Collection) ITypeCollection(iid int32) (r ITypeCollection) {
	if it := this.IType(iid); it != nil {
		r, _ = it.(ITypeCollection)
	}
	return
}

func (this *Collection) BulkWrite() dataset.BulkWrite {
	if this.bulkWrite == nil {
		this.bulkWrite = this.model.BulkWrite(this.Updater)
	}
	return this.bulkWrite
}

func (this *Collection) mayChange(op *operator.Operator) (err error) {
	it := this.ITypeCollection(op.IID)
	if it == nil {
		return ErrITypeNotExist(op.IID)
	}
	op.Bag = it.ID()
	if listen, ok := it.(ITypeListener); ok {
		listen.Listener(this.Updater, op)
	}
	if op.Type == operator.TypesDrop || op.Type == operator.TypesResolve {
		return nil
	}
	//可以堆叠道具
	if op.OID == "" && it.Stacked(op.IID) {
		op.OID, err = it.ObjectId(this.Updater, op.IID)
	}
	if err != nil {
		return
	}
	if op.OID != "" {
		this.statement.Select(op.OID)
	}

	return
}

func (this *Collection) operator(t operator.Types, k any, v int64, r any) {
	if err := this.Updater.WriteAble(); err != nil {
		return
	}
	op := operator.New(t, v, r)
	switch d := k.(type) {
	case string:
		op.OID = d
		op.IID, this.Updater.Error = Config.ParseId(this.Updater, op.OID)
	default:
		op.IID = dataset.ParseInt32(k)
	}

	if this.Updater.Error != nil {
		return
	}
	if err := this.mayChange(op); err != nil {
		this.Updater.Error = err
		return
	}
	this.statement.Operator(op)
}

func (this *Collection) ObjectId(key any) (oid string, err error) {
	if v, ok := key.(string); ok {
		return v, nil
	}
	iid := dataset.ParseInt32(key)
	it := this.ITypeCollection(iid)
	if it == nil {
		return "", fmt.Errorf("IType unknown:%v", iid)
	}
	if !it.Stacked(iid) {
		return "", ErrObjectIdEmpty(iid)
	}
	oid, err = it.ObjectId(this.Updater, iid)
	if err == nil && oid == "" {
		err = ErrUnableUseIIDOperation
	}
	return
}
