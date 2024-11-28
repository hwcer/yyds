package config

import (
	"github.com/hwcer/cosgo/logger"
)

// 保存整理过后的配置或者概率表

func (c *config) GetProcess(name string) any {
	return c.preprocess[name]
}

func (c *config) SetProcess(name string, value any) {
	if _, ok := c.preprocess[name]; ok {
		logger.Error("SetProcess name exist:%s", name)
		return
	}
	p := map[string]any{}
	for k, v := range c.preprocess {
		p[k] = v
	}
	p[name] = value
	c.preprocess = p
}
