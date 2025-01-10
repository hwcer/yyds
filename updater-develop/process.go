package updater

import (
	"github.com/hwcer/cosgo/logger"
	"strings"
)

type processCreator func(updater *Updater) any

var processDefault = map[string]processCreator{}

func RegisterGlobalProcess(name string, creator processCreator) {
	name = strings.ToLower(name)
	if processDefault[name] != nil {
		logger.Alert("player handle register already registered:%v", name)
	} else {
		processDefault[name] = creator
	}
}

type Process map[string]any

func (pro Process) Has(name string) bool {
	_, ok := pro[name]
	return ok
}
func (pro Process) Try(u *Updater, name string, f processCreator) {
	if _, ok := pro[name]; !ok {
		pro[name] = f(u)
	}
}

func (pro Process) Set(name string, value any) bool {
	if _, ok := pro[name]; ok {
		return false
	}
	pro[name] = value
	return true
}
func (pro Process) Get(name string) any {
	return pro[name]
}

func (pro Process) Delete(name string) {
	delete(pro, name)
}
