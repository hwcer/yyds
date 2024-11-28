package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosgo/schema"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

var locker sync.RWMutex

func GetIMax(iid int32) (r int64) {
	return Config.GetIMax(iid)
}
func GetIType(iid int32) (r int32) {
	return Config.GetIType(iid)
}

func Reload(c any) (err error) {
	locker.Lock()
	defer locker.Unlock()

	files, err := os.Stat(cosgo.Abs(options.Options.Config))
	if err != nil {
		return
	}
	if files.IsDir() {
		err = ReloadFromMultiple(c)
	} else {
		err = ReloadFromSingle(c)
	}
	if err != nil {
		return
	}
	if !verifyConfigData(c) {
		if cosgo.Debug() {
			logger.Alert("配置检查未通过!请检查日志")
		} else {
			return errors.New("配置检查未通过!请检查日志")
		}
	}
	for _, v := range hvs {
		v.Handle(c)
	}
	return
}

// ReloadFromSingle 从单个文件中加载配置
func ReloadFromSingle(c any) (err error) {
	file := cosgo.Abs(options.Options.Config)
	var in []byte
	if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
		in, err = getDataFromUrl(file)
	} else if file != "" {
		in, err = os.ReadFile(cosgo.Abs(file))
	} else {
		return errors.New("静态数据地址为空")
	}

	if err != nil {
		return
	}
	if ext := strings.ToLower(filepath.Ext(file)); ext == ".json" {
		err = json.Unmarshal(in, c)
	} else {
		err = fmt.Errorf("配置格式暂时不支持:%v", ext)
	}
	if err != nil {
		logger.Alert("无法解析静态数据,可能是版本不匹配:%v", file)
	}
	return
}

// ReloadFromMultiple 从多个文件中加载数据
func ReloadFromMultiple(c any) (err error) {
	dir := cosgo.Abs(options.Options.Config)
	//vf := reflect.Indirect(reflect.ValueOf(gd.Data))
	modelType := schema.Kind(c)
	bytes := strings.Builder{}
	bytes.WriteString("{")

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		if ast.IsExported(field.Name) {
			file := filepath.Join(dir, field.Name+".json")
			//logger.Trace("%v", file)
			if in, err := os.ReadFile(file); err != nil {
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
	if err = json.Unmarshal([]byte(s), c); err != nil {
		logger.Alert("无法解析静态数据,可能是版本不匹配")
		return err
	}

	return
}

func verifyConfigData(data any) (result bool) {
	result = true
	for _, v := range hvs {
		if errs := v.Verify(data); len(errs) > 0 {
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

func getDataFromUrl(url string) (b []byte, err error) {
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
	//b, err = request.Request(http.MethodGet, address, nil)
	//if err != nil {
	//	logger.Trace("加载远程配置错误：%v", address)
	//}

	err = errors.New("无法加载远程配置")
	return
}
