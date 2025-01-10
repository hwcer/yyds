package updater

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/schema"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
)

type valuesModel interface {
	Getter(u *Updater, data *dataset.Values, keys []int32) (err error) //获取数据接口
	Setter(u *Updater, data dataset.Data, expire int64) error          //保存数据接口
}

// Values 数字型键值对
type Values struct {
	statement
	name    string //model database name
	model   valuesModel
	dirty   dataset.Data //需要写入数据的数据
	dataset *dataset.Values
}

func NewValues(u *Updater, model any) Handle {
	r := &Values{}
	r.model = model.(valuesModel)
	r.statement = *newStatement(u, r.operator, r.Has)
	if sch, err := schema.Parse(model); err == nil {
		r.name = sch.Table
	} else {
		logger.Fatal(err)
	}
	return r
}

func (this *Values) Parser() Parser {
	return ParserTypeValues
}

func (this *Values) loading(ram RAMType) error {
	if this.dataset == nil {
		this.dataset = dataset.NewValues()
	}
	this.statement.ram = ram
	if !this.statement.loader && (this.statement.ram == RAMTypeMaybe || this.statement.ram == RAMTypeAlways) {
		this.statement.loader = true
		this.Updater.Error = this.model.Getter(this.Updater, this.dataset, nil)
	}
	return this.Updater.Error
}

func (this *Values) save() (err error) {
	if this.Updater.Async {
		return
	}
	dirty := this.Dirty()
	expire := this.dataset.Save(dirty)
	if len(dirty) == 0 {
		return nil
	}
	if err = this.model.Setter(this.statement.Updater, dirty, expire); err == nil {
		this.dirty = nil
	}
	return
}

// reset 运行时开始时
func (this *Values) reset() {
	this.statement.reset()
	if this.dataset == nil {
		this.dataset = dataset.NewValues()
	}
	if expire := this.dataset.Expire(); expire > 0 && expire < this.Updater.Now.Unix() {
		if this.Updater.Error = this.save(); this.Updater.Error != nil {
			logger.Alert("保存数据失败,name:%v,data:%v\n%v", this.name, this.dataset, this.Updater.Error)
		} else {
			this.dataset = dataset.NewValues()
			this.statement.loader = false
			this.Updater.Error = this.loading(this.statement.ram)
		}
	}
}

// release 运行时释放
func (this *Values) release() {
	this.statement.release()
	if !this.Updater.Async {
		if this.statement.ram == RAMTypeNone {
			this.dataset = nil
		}
	}
}

// 关闭时执行,玩家下线
func (this *Values) destroy() (err error) {
	return this.save()
}
func (this *Values) Len() int {
	return this.dataset.Len()
}
func (this *Values) Has(k any) bool {
	return this.dataset.Has(dataset.ParseInt32(k))
}

func (this *Values) Get(k any) (r any) {
	if id, ok := dataset.TryParseInt32(k); ok {
		r, _ = this.dataset.Get(id)
	}
	return
}
func (this *Values) Val(k any) (r int64) {
	if id, ok := dataset.TryParseInt32(k); ok {
		r = this.dataset.Val(id)
	}
	return
}

func (this *Values) All() dataset.Data {
	return this.dataset.All()
}

// Set 设置
// Set(k int32,v int64)
func (this *Values) Set(k any, v ...any) {
	switch len(v) {
	case 1:
		this.operator(operator.TypesSet, k, 0, dataset.ParseInt64(v[0]))
	default:
		this.Updater.Error = ErrArgsIllegal(k, v)
	}
}

// Select 指定需要从数据库更新的字段
func (this *Values) Select(keys ...any) {
	if this.ram == RAMTypeAlways {
		return
	}
	for _, k := range keys {
		if iid, ok := dataset.TryParseInt32(k); ok {
			this.statement.Select(iid)
		}
	}
}

func (this *Values) Data() (err error) {
	if this.Updater.Error != nil {
		return this.Updater.Error
	}
	if len(this.keys) == 0 {
		return nil
	}
	keys := this.keys.ToInt32()
	if err = this.model.Getter(this.statement.Updater, this.dataset, keys); err == nil {
		this.statement.date()
	}
	return
}

func (this *Values) verify() (err error) {
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

func (this *Values) submit() (err error) {
	if err = this.Updater.WriteAble(); err != nil {
		return
	}
	this.statement.submit()
	if err = this.save(); err != nil && this.ram != RAMTypeNone {
		logger.Alert("数据库[%v]同步数据错误,等待下次同步:%v", this.name, err)
		err = nil
	}
	return
}

func (this *Values) Range(f func(int32, int64) bool) {
	this.dataset.Range(f)
}

func (this *Values) IType(iid int32) IType {
	if h, ok := this.model.(modelIType); ok {
		v := h.IType(iid)
		return itypesDict[v]
	} else {
		return this.Updater.IType(iid)
	}
}

func (this *Values) Dirty() (r dataset.Data) {
	if this.dirty == nil {
		this.dirty = dataset.Data{}
	}
	return this.dirty
}

func (this *Values) operator(t operator.Types, k any, v int64, r any) {
	if err := this.Updater.WriteAble(); err != nil {
		return
	}
	id, ok := dataset.TryParseInt32(k)
	if !ok {
		_ = this.Errorf("updater Hash Operator key must int32:%v", k)
		return
	}
	//if t != operator.TypesDel {
	//	if _, ok = dataset.TryParseInt64(v); !ok {
	//		_ = this.Errorf("updater Hash Operator val must int64:%v", v)
	//		return
	//	}
	//}
	op := operator.New(t, v, r)
	op.IID = id
	this.statement.Select(id)
	it := this.IType(op.IID)
	if it == nil {
		logger.Debug("IType not exist:%v", op.IID)
		return
	}
	op.Bag = it.ID()
	if listen, ok := it.(ITypeListener); ok {
		listen.Listener(this.Updater, op)
	}
	this.statement.Operator(op)
}
