package config

import "github.com/hwcer/yyds/options"

var Config = &CS{ITypes: ITypes{}, Process: Process{}}

func Is(iid int32, it int32) bool {
	return Config.ITypes.GetIType(iid) == it
}

func Has(k int32) bool {
	_, ok := Config.ITypes[k]
	return ok
}

func GetIMax(iid int32) (r int64) {
	return options.Setting.GetIMax(iid)
}
func GetName(iid int32) (r string) {
	if i := Config.ITypes.get(iid); i != nil {
		r = i.Name
	}
	return
}
func GetIType(iid int32) (r int32) {
	return options.Setting.GetIType(iid)
}

func Reload(data any, path string) (err error) {
	return Config.Reload(data, path)
}
