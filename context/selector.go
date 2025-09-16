package context

import "github.com/hwcer/yyds/options"

// Selector 微服务设置器
func (this *Context) Selector() *Selector {
	return &Selector{Context: this}
}

type Selector struct {
	*Context
}

func (this *Selector) Set(k, v string) {
	name := options.GetServiceSelectorAddress(k)
	this.SetMetadata(name, v)
}

func (this *Selector) Remove(k string) {
	name := options.GetServiceSelectorAddress(k)
	this.SetMetadata(name, "")
}
