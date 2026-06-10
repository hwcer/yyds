package verify

import "github.com/hwcer/updater"

// MethodHandle 自定义取值方法接口，用于 ConditionMethod 类型
type MethodHandle interface {
	Value(u *updater.Updater, value Value) int64
	Target(u *updater.Updater, value Value)
}

// MethodValue 简单取值函数，通过 SetMethod 注册后自动包装为 MethodHandle
type MethodValue func(u *updater.Updater, value Value) int64

var methodRegister = map[int32]MethodHandle{}

// SetMethod 注册简单取值函数（无需预加载数据）
func SetMethod(key int32, fun MethodValue) {
	methodRegister[key] = &defaultMethodValue{fun: fun}
}

// SetMethodHandle 注册完整的自定义方法（支持预加载）
func SetMethodHandle(key int32, handle MethodHandle) {
	methodRegister[key] = handle
}

// GetMethod 获取已注册的自定义方法
func GetMethod(key int32) MethodHandle {
	return methodRegister[key]
}

type defaultMethodValue struct {
	fun MethodValue
}

func (this *defaultMethodValue) Value(u *updater.Updater, value Value) int64 {
	return this.fun(u, value)
}
func (this *defaultMethodValue) Target(u *updater.Updater, value Value) {}
