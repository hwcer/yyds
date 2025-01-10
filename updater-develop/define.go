package updater

import (
	"github.com/hwcer/updater/operator"
)

var cacheFilterRule = map[int32]any{}

var Config = struct {
	IMax    func(iid int32) int64                                     //通过道具iid查找上限
	IType   func(iid int32) int32                                     //通过道具iid查找IType ID
	ParseId func(adapter *Updater, oid string) (iid int32, err error) //解析OID获得IID
	Filter  func(*operator.Operator) bool                             //过滤cache,返回false时不返回给前端
}{
	Filter: cacheFilterHandle,
}

func cacheFilterHandle(o *operator.Operator) bool {
	rule, ok := cacheFilterRule[o.Bag]
	if !ok {
		return true
	}
	switch v := rule.(type) {
	case bool:
		return v
	case func(*operator.Operator) bool:
		return v(o)
	default:
		return true
	}
}

// SetCacheFilterRule 设置Updater cache过滤规则
// rule bool
// rule func(*operator.Operator) bool  自定义规则
func SetCacheFilterRule(it int32, rule any) {
	cacheFilterRule[it] = rule
}

// IType 一个IType对于一种数据类型·
// 多种数据类型 可以用一种数据模型(model,一张表结构)
type IType interface {
	ID() int32 //IType 唯一标志
}

type ITypeCollection interface {
	IType
	New(u *Updater, op *operator.Operator) (item any, err error) //根据Operator信息生成新对象
	Stacked(int32) bool                                          //是否可以叠加
	ObjectId(u *Updater, iid int32) (oid string, err error)      //使用IID创建OID,仅限于可以叠加道具,不可以叠加道具返回空,使用NEW来创建
}

// ITypeResolve 自动分解,如果没有分解方式超出上限则使用系统默认方式（丢弃）处理
// Verify执行的一部分(Data之后Save之前)
// 使用Resolve前，需要使用ITypeListener监听将可能分解成的道具ID使用adapter.Select预读数据
// 使用Resolve时需要关联IMax指定道具上限
type ITypeResolve interface {
	Resolve(u *Updater, iid int32, val int64) error
}

type ITypeListener interface {
	Listener(u *Updater, op *operator.Operator)
}

// ModelIType 获取默认IType,仅仅doc模型使用
//type ModelIType interface {
//	IType() int32
//}

// ModelListener 监听数据变化
//type ModelListener interface {
//	Listener(u *Updater, op *operator.Operator)
//}

type Keys map[any]struct{}

func (this Keys) Has(k any) (ok bool) {
	_, ok = this[k]
	return
}

func (this Keys) Remove(k any) {
	delete(this, k)
}

func (this Keys) ToString() (r []string) {
	for k, _ := range this {
		if sk, ok := k.(string); ok {
			r = append(r, sk)
		}
	}
	return
}

func (this Keys) ToInt32() (r []int32) {
	for k, _ := range this {
		if ik, ok := k.(int32); ok {
			r = append(r, ik)
		}
	}
	return
}

//func (this Keys) Keys() (r []string) {
//	for k, _ := range this {
//		if sk, ok := k.(string); ok {
//			r = append(r, sk)
//		}
//	}
//	return
//}

func (this Keys) Merge(src Keys) {
	for k, _ := range src {
		this[k] = struct{}{}
	}
}

func (this Keys) Select(ks ...any) {
	for _, k := range ks {
		this[k] = struct{}{}
	}
}

//type documentKeys map[string]any

//type Dirty map[string]any
//
//func (this Dirty) Get(k string) any {
//	return this[k]
//}
//
//func (this Dirty) Has(k string) bool {
//	if _, ok := this[k]; ok {
//		return true
//	}
//	return false
//}
//
//func (this Dirty) Keys() (r []string) {
//	for k, _ := range this {
//		r = append(r, k)
//	}
//	return
//}
//
//func (this Dirty) Merge(src Dirty) {
//	for k, v := range src {
//		this[k] = v
//	}
//}
