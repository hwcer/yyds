package yyds

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/schema"
	"github.com/hwcer/yyds/options"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var Config = &CS{ITypes: configITypes{}, Process: configProcess{}}
var configLocker sync.RWMutex
var configHandles []configHandle

type iMax interface {
	GetIMax() int32
}
type iType interface {
	GetIType() int32
}
type configIType struct {
	Name  string
	IMax  int32
	IType int32
}

// configHandle 检查或者预处理接口
type configHandle interface {
	Handle(c *CS, d any)         //配置预处理
	Verify(c *CS, d any) []error //配置检查
}

type CS struct {
	ITypes  configITypes
	Process configProcess
}

// Register 注册配置检查程序
func (c *CS) Register(i ...configHandle) {
	configHandles = append(configHandles, i...)
}

//func (c *CS) GetIMax(iid int32) (r int64) {
//	return cfg.GetIMax(iid)
//}
//func (c *CS) GetIType(iid int32) (r int32) {
//	return cfg.GetIType(iid)
//}
//func (c *CS) GetProcess(name string) any {
//	return cfg.Process.Get(name)
//}

func (*CS) Reload(data any) (err error) {
	configLocker.Lock()
	defer configLocker.Unlock()
	c := &CS{ITypes: configITypes{}, Process: configProcess{}}

	files, err := os.Stat(cosgo.Abs(options.Options.Data))
	if err != nil {
		return
	}
	if files.IsDir() {
		err = c.ReloadFromMultiple(data)
	} else {
		err = c.ReloadFromSingle(data)
	}
	if err != nil {
		return
	}
	if !c.verifyConfigData(data) {
		if cosgo.Debug() {
			logger.Alert("配置检查未通过!请检查日志")
		} else {
			return errors.New("配置检查未通过!请检查日志")
		}
	}
	for _, v := range configHandles {
		v.Handle(c, data)
	}
	Config = c
	return
}

// ReloadFromSingle 从单个文件中加载配置
func (c *CS) ReloadFromSingle(d any) (err error) {
	file := cosgo.Abs(options.Options.Data)
	var in []byte
	if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
		in, err = c.getDataFromUrl(file)
	} else if file != "" {
		in, err = os.ReadFile(cosgo.Abs(file))
	} else {
		return errors.New("静态数据地址为空")
	}

	if err != nil {
		return
	}
	if ext := strings.ToLower(filepath.Ext(file)); ext == ".json" {
		err = json.Unmarshal(in, d)
	} else {
		err = fmt.Errorf("配置格式暂时不支持:%v", ext)
	}
	if err != nil {
		logger.Alert("无法解析静态数据,可能是版本不匹配:%v", file)
	}
	return
}

// ReloadFromMultiple 从多个文件中加载数据
func (c *CS) ReloadFromMultiple(d any) (err error) {
	dir := cosgo.Abs(options.Options.Data)
	//vf := reflect.Indirect(reflect.ValueOf(gd.Data))
	modelType := schema.Kind(d)
	bytes := strings.Builder{}
	bytes.WriteString("{")

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		if ast.IsExported(field.Name) {
			file := filepath.Join(dir, field.Name+".json")
			//logger.Trace("%v", file)
			var in []byte
			if in, err = os.ReadFile(file); err != nil {
				logger.Alert("加载配置数据失败,文件:%v", file)
				logger.Alert("加载配置数据失败,原因:%v", err)
			} else {
				bytes.WriteString(fmt.Sprintf(`"%v":%v`, field.Name, string(in)))
				bytes.WriteString(",")
			}
		}
	}

	s := strings.TrimSuffix(bytes.String(), ",")
	s = s + "}"
	//logger.Trace("%v", s)
	if err = json.Unmarshal([]byte(s), d); err != nil {
		logger.Alert("无法解析静态数据,可能是版本不匹配")
		return err
	}

	return
}

func (c *CS) verifyConfigData(data any) (result bool) {
	result = true
	for _, v := range configHandles {
		if errs := v.Verify(c, data); len(errs) > 0 {
			result = false
			vf := reflect.TypeOf(v)
			var name string
			if vf.Kind() == reflect.Ptr {
				name = vf.Elem().Name()
			} else {
				name = vf.Name()
			}
			for _, err := range errs {
				fmt.Printf("配置检查错误[%v]:%v\n", name, err)
			}
		}
	}
	return
}

func (c *CS) getDataFromUrl(url string) (b []byte, err error) {
	//if err = request.Get(url, &Version); err != nil {
	//	return
	//}
	//
	//file := fmt.Sprintf(Version.StaticDataBuffer, Version.StaticDataVersion)
	//arr := strings.Split(url, "/")
	//arr[len(arr)-1] = strings.TrimPrefix(file, "/")
	//
	////logger.Debug("url:%v", strings.Join(arr, "/"))
	//address := strings.Join(arr, "/")
	//b, err = request.OnSend(http.MethodGet, address, nil)
	//if err != nil {
	//	logger.Trace("加载远程配置错误：%v", address)
	//}

	err = errors.New("无法加载远程配置")
	return
}

type configITypes map[int32]*configIType

func (its configITypes) set(k int32, v *configIType) {
	its[k] = v
}
func (its configITypes) get(k int32) *configIType {
	return its[k]
}

func (its configITypes) Add(k int32, iType int32, iMax int32, name string) {
	it := &configIType{Name: name, IType: iType, IMax: iMax}
	its.set(k, it)
}

func (its configITypes) Has(k int32) bool {
	_, ok := its[k]
	return ok
}

func (its configITypes) GetIMax(iid int32) (r int64) {
	if i := its.get(iid); i != nil {
		r = int64(i.IMax)
	}
	return
}

func (its configITypes) GetIType(iid int32) (r int32) {
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

func (its configITypes) Parse(name string, items any, iType int32, iMax int32) (errs []error) {
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
		if x := its.get(id); x != nil {
			errs = append(errs, fmt.Errorf("道具ID重复,%v[%v]=%v[%v]", name, id, x.Name, id))
		}

		it := &configIType{Name: name, IType: iType}
		its.set(id, it)
		if it.IMax = its.reflectIMax(id, i); it.IMax == 0 {
			it.IMax = iMax
		}
		if iType != 0 {
			if strings.HasPrefix(strconv.Itoa(int(id)), sit) {
				it.IType = iType
			} else {
				errs = append(errs, fmt.Errorf("%v[%v]必须以itype[%v]开头", name, id, iType))
			}
		} else {
			if it.IType = its.reflectIType(id, i); it.IType == 0 {
				errs = append(errs, fmt.Errorf("IType为空:%v[%v]", name, id))
			}
		}
	}
	return
}

func (its configITypes) reflectIType(id int32, i interface{}) int32 {
	if v, ok := i.(iType); ok {
		return v.GetIType()
	}
	s := strconv.Itoa(int(id))
	v, _ := strconv.Atoi(s[0:2])
	return int32(v)
}
func (its configITypes) reflectIMax(id int32, i interface{}) int32 {
	if v, ok := i.(iMax); ok {
		return v.GetIMax()
	}
	return 0
}

// 保存整理过后的配置或者概率表

type configProcess map[string]any

func (p configProcess) Get(name string) any {
	return p[name]
}

func (p configProcess) Set(name string, value any) {
	if _, ok := p[name]; ok {
		logger.Error("SetProcess name exist:%s", name)
		return
	}
	p[name] = value
}
