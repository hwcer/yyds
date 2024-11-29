package config

import (
	"github.com/hwcer/cosgo/logger"
)

// 保存整理过后的配置或者概率表

type Process map[string]any

func (p Process) Get(name string) any {
	return p[name]
}

func (p Process) Set(name string, value any) {
	if _, ok := p[name]; ok {
		logger.Error("SetProcess name exist:%s", name)
		return
	}
	p[name] = value
}
