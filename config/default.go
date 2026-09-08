package config

func Is(iid int32, it ...int32) bool {
	return Load().ITypes.Is(iid, it...)
}

func Has(k int32) bool {
	return Load().ITypes.Has(k)
}

func GetIMax(iid int32) (r int64) {
	return Load().ITypes.GetIMax(iid)
}

func GetIType(iid int32) (r int32) {
	return Load().ITypes.GetIType(iid)
}

func GetName(iid int32) (r string) {
	return Load().ITypes.GetName(iid)
}
