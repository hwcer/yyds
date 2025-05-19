package config

import "github.com/hwcer/logger"

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
