package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/schema"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/options"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

// 静态数据加载，热更

// 保存整理过后的配置或者概率表
var mutex sync.RWMutex

type CS struct {
	ITypes
	Process Process
}

// Register 注册配置检查程序
func (cs *CS) Register(i ...Handle) {
	handles = append(handles, i...)
}

func (cs *CS) Reload(data any) (err error) {
	mutex.Lock()
	defer mutex.Unlock()
	c := &CS{ITypes: ITypes{}, Process: Process{}}

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
	if !c.verify(data) {
		if cosgo.Debug() {
			logger.Alert("配置检查未通过!请检查日志")
		} else {
			return errors.New("配置检查未通过!请检查日志")
		}
	}
	for _, v := range handles {
		v.Handle(c, data)
	}
	Config.ITypes, Config.Process = c.ITypes, c.Process
	return
}

// ReloadFromSingle 从单个文件中加载配置
func (cs *CS) ReloadFromSingle(d any) (err error) {
	file := cosgo.Abs(options.Options.Data)
	var in []byte
	if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
		in, err = cs.getDataFromUrl(file)
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
func (cs *CS) ReloadFromMultiple(d any) (err error) {
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

func (cs *CS) verify(data any) (result bool) {
	result = true
	for _, v := range handles {
		if errs := v.Verify(cs, data); len(errs) > 0 {
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

func (cs *CS) getDataFromUrl(url string) (b []byte, err error) {
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
