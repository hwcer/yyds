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
		f = Filter
	}
	return f(tar, args)
}

// Filter 默认全局参数过滤
var Filter = func(tar, args []int32) bool {
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
