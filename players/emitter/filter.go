package emitter

// 条件过滤器
var Filters = filters{}

type filters map[int32]FilterFunc

type FilterFunc func(tar, args []int32) bool //条件过滤器

func (fs filters) Register(t int32, f FilterFunc) {
	fs[t] = f
}

func (fs filters) Compare(t int32, tar, args []int32) bool {
	var f FilterFunc
	if v, ok := Filters[t]; ok {
		f = v
	}
	if f == nil {
		f = defaultFilter
	}
	return f(tar, args)
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
