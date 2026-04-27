package options

var Setting = struct {
	Renewal  string                    //启用跨天
	GetIType func(iid int32) (r int32) //替代config.GetIType
}{
	Renewal: "/role/renewal",
}
