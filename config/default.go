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

// GetProcess 读取当前快照里某 Handle 的预处理产物(概率表、索引等)，未注册返回 nil。
//
// 零散的单点读取用本函数即可；一次请求内要 Payload/Process/ITypes **同世代**时，
// 请从 context.Context.Config(请求入口取的整份快照)读，热更落在请求中间也不会前后不一致。
func GetProcess(name string) any {
	return Load().Process.Get(name)
}
