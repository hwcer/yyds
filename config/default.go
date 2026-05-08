package config

var Config = &CS{ITypes: ITypes{}, Process: Process{}}

func Is(iid int32, it int32) bool {
	return Config.ITypes.GetIType(iid) == it
}

func Has(k int32) bool {
	_, ok := Config.ITypes[k]
	return ok
}

func GetIMax(iid int32) (r int64) {
	return Config.GetIMax(iid)
}

func GetIType(iid int32) (r int32) {
	return Config.GetIType(iid)
}

func GetName(iid int32) (r string) {
	if i := Config.ITypes.get(iid); i != nil {
		r = i.Name
	}
	return
}

func Reload(data any, path string) (err error) {
	return Config.Reload(data, path)
}
