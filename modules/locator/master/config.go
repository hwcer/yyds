package master

import (
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosweb"
)

func init() {
	_ = Service.Register(&Config{})
}

type Config struct {
}

func (this *Config) Caller(node *registry.Node, c *cosweb.Context) interface{} {
	method := node.Method()
	f := method.(func(*Config, *cosweb.Context) interface{})
	return f(this, c)
}

// Update 配置更新，需要全服广播
func (this *Config) Update(c *cosweb.Context) interface{} {
	msg := make(map[string]interface{})
	msg["config"] = 1
	if err := Broadcast("S2CNotify", msg, nil); err != nil {
		return values.Error(err)
	}
	return "ok"
}
