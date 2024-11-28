package emitter

// 条件过滤器
var filters = map[int32]Filter{}

type Filter func(tar, args []int32) bool //条件过滤器

func Register(t int32, f Filter) {
	filters[t] = f
}

func Require(t int32) Filter {
	if v, ok := filters[t]; ok {
		return v
	}
	return defaultFilter
}

func defaultFilter(tar, args []int32) bool {
	if len(tar) > len(args) {
		return false
	}
	for i, v := range tar {
		if v != args[i] {
			return false
		}
	}
	return true
}
