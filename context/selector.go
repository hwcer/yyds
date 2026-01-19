package context

import (
	"github.com/hwcer/gateway/gwcfg"
)

// Selector 微服务设置器
func (this *Context) Selector() *Selector {
	return &Selector{Context: this}
}

type Selector struct {
	*Context
}

func (this *Selector) Set(k, v string) {
	name := gwcfg.GetServiceSelectorAddress(k)
	this.SetMetadata(name, v)
}

func (this *Selector) Remove(k string) {
	name := gwcfg.GetServiceSelectorAddress(k)
	this.SetMetadata(name, "")
}
