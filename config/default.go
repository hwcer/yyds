package config

import "strconv"

var Config = &CS{ITypes: ITypes{}, Process: Process{}}

func Is(iid int32, it int32) bool {
	return Config.ITypes.GetIType(iid) == it
}

func Has(k int32) bool {
	_, ok := Config.ITypes[k]
	return ok
}

func GetIMax(iid int32) (r int64) {
	if i := Config.ITypes.get(iid); i != nil {
		r = int64(i.IMax)
	}
	return
}
func GetName(iid int32) (r string) {
	if i := Config.ITypes.get(iid); i != nil {
		r = i.Name
	}
	return
}
func GetIType(iid int32) (r int32) {
	if iid < 10 {
		return 0
	}
	if i := Config.ITypes.get(iid); i != nil {
		r = i.IType
	} else {
		s := strconv.Itoa(int(iid))
		v, _ := strconv.Atoi(s[0:2])
		r = int32(v)
	}
	return
}
