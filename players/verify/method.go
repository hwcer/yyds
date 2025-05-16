package verify

import "github.com/hwcer/updater"

//使用自定义方法获取值

type MethodHandle interface {
	Value(u *updater.Updater, value Value) int64
	Target(u *updater.Updater, value Value)
}
type MethodValue func(u *updater.Updater, value Value) int64

var methodRegister = map[int32]MethodHandle{}

func SetMethod(key int32, fun MethodValue) {
	methodRegister[key] = &defaultMethodValue{fun: fun}
}

func SetMethodHandle(key int32, handle MethodHandle) {
	methodRegister[key] = handle
}

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
