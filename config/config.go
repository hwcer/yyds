package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/schema"
	"github.com/hwcer/logger"
)

// 静态数据加载，热更
//
// 发布模型是 copy-on-write 快照：Reload 在私有 Snapshot 上完成全部构建(读文件、verify、
// 逐个 Handle 预处理)，最后通过一次 atomic Store 整体发布；读者(请求 goroutine)
// 无锁读取，任意时刻拿到的都是某一世代的完整快照，旧快照发布后永不修改。
//
// 🔴 快照一旦发布，不得原地写 ITypes/Process 里的任何 map —— 那是 concurrent
// map write(runtime fatal，不可 recover)。要更新数据就构建新快照整体发布。

var mutex sync.RWMutex //仅串行化 Reload 自身；读者无锁走 atomic 快照

// snap 当前配置快照。
var snap atomic.Pointer[Snapshot]

func init() {
	snap.Store(&Snapshot{ITypes: ITypes{}, Process: Process{}})
}

// CS 是 Snapshot 的旧名别名，仅为兼容存量引用；新代码一律用 Snapshot。
type CS = Snapshot

// Snapshot 一世代配置快照：IType 注册表 + 各 Handle 的预处理产物(Process) +
// 业务静态数据(Payload)。
// 由 Reload 构建，发布后只读。Handle 接口以 *Snapshot 收发(其 d 参数即 c.Payload)，
// 业务实现无需感知发布细节。
type Snapshot struct {
	ITypes
	Process Process
	// Payload 业务方传给 Reload 的静态数据对象(通常是 xlsx 导出表的解析结构体)，
	// 随快照整体原子发布 —— 业务侧不再需要自建全局指针/发布机制。
	Payload any
}

// Load 返回当前快照。快照发布后不可变，可放心持有；
// 同一段逻辑要多次访问时取一次局部变量即可，视图天然一致。
func Load() *Snapshot {
	return snap.Load()
}

// Reload 重新加载静态数据并原子发布新快照。
// 构建全程在私有 Snapshot 上进行，任一步失败都不影响线上正在使用的旧快照。
func Reload(payload any, path string) (err error) {
	mutex.Lock()
	defer mutex.Unlock()
	//🔴 不能原地复用调用方的 payload：已发布旧快照的 Payload 就是它,
	//ReloadFromSingle/Multiple 的 json.Unmarshal 会原地写它,与并发读者
	//构成数据竞争(-race 实报,见 reload_race_test)。每次热更为 payload
	//建新实例再挂到新快照上——文件是完整数据源,旧快照发布后不可变。
	//payload 必须是非 nil 指针(原本 json.Unmarshal 也要求指针)
	rv := reflect.ValueOf(payload)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("config.Reload: payload 必须是非 nil 指针")
	}
	d := reflect.New(rv.Type().Elem()).Interface()
	//Payload 与 ITypes/Process 同快照发布：读者拿到的业务数据与派生表永远同世代。
	c := &Snapshot{ITypes: ITypes{}, Process: Process{}, Payload: d}
	path = cosgo.Abs(path)
	files, err := os.Stat(path)
	if err != nil {
		return
	}
	if files.IsDir() {
		err = c.ReloadFromMultiple(d, path)
	} else {
		err = c.ReloadFromSingle(d, path)
	}
	if err != nil {
		return
	}
	if !c.verify(d) {
		if cosgo.Debug() {
			logger.Alert("配置检查未通过!请检查日志")
		} else {
			return errors.New("配置检查未通过!请检查日志")
		}
	}
	for _, v := range handles {
		v.Handle(c, d)
	}
	snap.Store(c)
	return
}

// ReloadFromSingle 从单个文件中加载配置
func (cs *Snapshot) ReloadFromSingle(d any, file string) (err error) {
	var in []byte
	if file != "" {
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
func (cs *Snapshot) ReloadFromMultiple(d any, dir string) error {
	//vf := reflect.Indirect(reflect.ValueOf(gd.Data))
	modelType, err := schema.Parse(d)
	if err != nil {
		return err
	}
	bytes := strings.Builder{}
	bytes.WriteString("{")

	for _, field := range modelType.Fields {
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

	return nil
}

func (cs *Snapshot) verify(data any) (result bool) {
	result = true
	for _, v := range handles {
		if errs := v.Verify(cs, data); len(errs) > 0 {
			result = false
			vf := reflect.TypeOf(v)
			var name string
			if vf.Kind() == reflect.Pointer {
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
