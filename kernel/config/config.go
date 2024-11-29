package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var cfg = New()

type iMax interface {
	GetIMax() int32
}
type iType interface {
	GetIType() int32
}

type IType struct {
	Name  string
	IMax  int32
	IType int32
}

func New() *Config {
	return &Config{dict: map[int32]*IType{}, Process: Process{}}
}

type Config struct {
	dict    map[int32]*IType
	Process Process
}

func (c *Config) Set(k int32, v *IType) {
	if c.dict == nil {
		c.dict = make(map[int32]*IType)
	}
	c.dict[k] = v
}

func (c *Config) Add(k int32, iType int32, iMax int32, name string) {
	if c.dict == nil {
		c.dict = make(map[int32]*IType)
	}
	it := &IType{Name: name, IType: iType, IMax: iMax}
	c.Set(k, it)
}

func (c *Config) Get(k int32) *IType {
	return c.dict[k]
}

func (c *Config) Has(k int32) bool {
	_, ok := c.dict[k]
	return ok
}

func (c *Config) GetIMax(iid int32) (r int64) {
	if i := c.Get(iid); i != nil {
		r = int64(i.IMax)
	}
	return
}

func (c *Config) GetIType(iid int32) (r int32) {
	if iid < 10 {
		return 0
	}
	if i := c.Get(iid); i != nil {
		r = i.IType
	} else {
		s := strconv.Itoa(int(iid))
		v, _ := strconv.Atoi(s[0:2])
		r = int32(v)
	}
	return
}

func (c *Config) Parse(name string, items any, iType int32, iMax int32) (errs []error) {
	rv := reflect.ValueOf(items)
	if rv.Kind() != reflect.Map {
		errs = append(errs, fmt.Errorf("%v 不是有效的map", name))
		return
	}
	sit := strconv.Itoa(int(iType))
	for _, k := range rv.MapKeys() {
		id, ok := k.Interface().(int32)
		if !ok {
			errs = append(errs, fmt.Errorf("%v的道具ID不是INT32", name))
			return
		}
		v := rv.MapIndex(k)
		i := v.Interface()
		if x := c.Get(id); x != nil {
			errs = append(errs, fmt.Errorf("道具ID重复,%v[%v]=%v[%v]", name, id, x.Name, id))
		}

		it := &IType{Name: name, IType: iType}
		c.Set(id, it)
		if it.IMax = c.reflectIMax(id, i); it.IMax == 0 {
			it.IMax = iMax
		}
		if iType != 0 {
			if strings.HasPrefix(strconv.Itoa(int(id)), sit) {
				it.IType = iType
			} else {
				errs = append(errs, fmt.Errorf("%v[%v]必须以itype[%v]开头", name, id, iType))
			}
		} else {
			if it.IType = c.reflectIType(id, i); it.IType == 0 {
				errs = append(errs, fmt.Errorf("IType为空:%v[%v]", name, id))
			}
		}
	}
	return
}

func (c *Config) reflectIType(id int32, i interface{}) int32 {
	if v, ok := i.(iType); ok {
		return v.GetIType()
	}
	s := strconv.Itoa(int(id))
	v, _ := strconv.Atoi(s[0:2])
	return int32(v)
}
func (c *Config) reflectIMax(id int32, i interface{}) int32 {
	if v, ok := i.(iMax); ok {
		return v.GetIMax()
	}
	return 0
}
