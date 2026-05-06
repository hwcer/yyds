package options

import "strconv"

var Setting = struct {
	Renewal  string //启用跨天
	GetIMax  func(iid int32) (r int64)
	GetIType func(iid int32) (r int32) //替代config.GetIType
}{
	Renewal:  "/role/renewal",
	GetIMax:  getIMaxDefault,
	GetIType: getITypeDefault,
}

func getIMaxDefault(iid int32) (r int64) {
	return 0
}

func getITypeDefault(iid int32) (r int32) {
	if iid < 10 {
		return 0
	}
	s := strconv.Itoa(int(iid))
	v, _ := strconv.Atoi(s[0:2])
	r = int32(v)
	return
}
