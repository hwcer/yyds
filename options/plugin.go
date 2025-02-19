package options

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/xserver"
	"os"
	"plugin"
	"sync"
)

var Plugin = &reload{}

type reload struct {
	dict   map[string]*registry.Node
	locker sync.Mutex
	loaded map[string]struct{} //已经加载过的补丁
}

func init() {
	Plugin.dict = make(map[string]*registry.Node)
	Plugin.loaded = make(map[string]struct{})
}

func (this *reload) register(service, handle string, fn any, prefix ...string) {
	var arr []string
	if handle != "" {
		arr = append(arr, handle)
	}
	if len(prefix) > 0 {
		arr = append(arr, prefix...)
	} else {
		arr = append(arr, "%v")
	}
	r := xserver.Registry()
	if !r.Has(service) {
		logger.Alert("register service %s not exist", service)
		return
	}
	s := r.Service(service)
	if node, err := s.Node(fn, arr...); err == nil {
		name := node.Service.Name() + "." + node.Name()
		this.dict[name] = node
	} else {
		logger.Alert(err)
	}
}

func (this *reload) Register(service string, fn any, prefix ...string) {
	this.register(service, Gate.Prefix, fn, prefix...)
}

// RegisterPrivate 注册只有内部机器才能访问的接口,用户无法通过网关访问
func (this *reload) RegisterPrivate(service string, fn any, prefix ...string) {
	this.register(service, "", fn, prefix...)
}

// Range 遍历接口，需要将此方法引入到main包中导出
func (this *reload) Range(h func(name string, node *registry.Node) bool) {
	for name, fn := range this.dict {
		if !h(name, fn) {
			break
		}
	}
}

// Get 获取所有接口，需要将此方法引入到main包中导出
func (this *reload) Get() map[string]*registry.Node {
	r := make(map[string]*registry.Node)
	for k, v := range this.dict {
		r[k] = v
	}
	return r
}

// Reload 热更新
func (this *reload) Reload(p string) (err error) {
	if p == "" {
		return values.Error("reload name is empty")
	}
	file := cosgo.Abs(p)
	if _, err = os.Stat(file); err != nil {
		return values.Errorf(0, "热更错误，已经放弃执行热更。\n热更文件:%v\n错误信息:%v\n", file, err)
	}
	this.locker.Lock()
	defer this.locker.Unlock()
	if _, ok := this.loaded[file]; ok {
		logger.Trace("已经加载过补丁，放弃执行热更:%v", file)
		return nil
	}
	defer func() {
		if err == nil {
			this.loaded[file] = struct{}{}
		}
	}()
	plug, err := plugin.Open(file)
	if err != nil {
		return err
	}

	i, err := plug.Lookup("Get")
	if err != nil {
		return err
	}
	getter, ok := i.(func() map[string]*registry.Node)
	if !ok {
		return values.Error("reload not found func Range")
	}
	apis := getter()
	return xserver.Reload(apis)
}
