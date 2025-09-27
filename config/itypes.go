package config

import (
	"fmt"
	"reflect"
	"strconv"
)

type IType struct {
	Name  string
	IMax  int32
	IType int32
}

type ITypes map[int32]*IType

func (its ITypes) set(k int32, v *IType) {
	its[k] = v
}
func (its ITypes) get(k int32) *IType {
	return its[k]
}

func (its ITypes) Add(k int32, iType int32, iMax int32, name string) {
	it := &IType{Name: name, IType: iType, IMax: iMax}
	its.set(k, it)
}

func (its ITypes) Is(iid int32, it ...int32) bool {
	i := its.GetIType(iid)
	if i == 0 {
		return false
	}
	for _, v := range it {
		if i == v {
			return true
		}
	}
	return false
}

func (its ITypes) Has(k int32) bool {
	_, ok := its[k]
	return ok
}

func (its ITypes) GetIMax(iid int32) (r int64) {
	if i := its.get(iid); i != nil {
		r = int64(i.IMax)
	}
	return
}
func (its ITypes) GetName(iid int32) (r string) {
	if i := its.get(iid); i != nil {
		r = i.Name
	}
	return
}
func (its ITypes) GetIType(iid int32) (r int32) {
	if iid < 10 {
		return 0
	}
	if i := its.get(iid); i != nil {
		r = i.IType
	} else {
		s := strconv.Itoa(int(iid))
		v, _ := strconv.Atoi(s[0:2])
		r = int32(v)
	}
	return
}

func (its ITypes) Parse(name string, items any, iType int32, iMax int32) (errs []error) {
	rv := reflect.ValueOf(items)
	if rv.Kind() != reflect.Map {
		errs = append(errs, fmt.Errorf("%v 不是有效的map", name))
		return
	}
	//sit := strconv.Itoa(int(iType))
	for _, k := range rv.MapKeys() {
		id, ok := k.Interface().(int32)
		if !ok {
			errs = append(errs, fmt.Errorf("%v的道具ID不是INT32", name))
			return
		}
		v := rv.MapIndex(k)
		i := v.Interface()
		if x := its.get(id); x != nil {
			errs = append(errs, fmt.Errorf("道具ID重复,%v[%v]=%v[%v]", name, id, x.Name, id))
		}

		it := &IType{Name: name, IType: iType}

		its.set(id, it)
		if it.Name = its.reflectIName(id, i); it.Name == "" {
			it.Name = name
		}
		if it.IMax = its.reflectIMax(id, i); it.IMax == 0 {
			it.IMax = iMax
		}

		if s := its.reflectIType(id, i); s != 0 {
			it.IType = s //配置设定类型
		} else if iType > 0 {
			it.IType = iType //统一类型
		} else {
			errs = append(errs, fmt.Errorf("IType为空:%v[%v]", name, id))
		}

	}
	return
}

func (its ITypes) reflectIType(id int32, i interface{}) int32 {
	if v, ok := i.(iType); ok {
		return v.GetIType()
	}
	s := strconv.Itoa(int(id))
	v, _ := strconv.Atoi(s[0:2])
	return int32(v)
}
func (its ITypes) reflectIMax(id int32, i interface{}) int32 {
	if v, ok := i.(iMax); ok {
		return v.GetIMax()
	}
	return 0
}

func (its ITypes) reflectIName(id int32, i interface{}) string {
	if v, ok := i.(iName); ok {
		return v.GetName()
	}
	return ""
}
