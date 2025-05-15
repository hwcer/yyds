package verify

import "github.com/hwcer/updater"

//使用自定义方法获取值

type Method interface {
	Value(u *updater.Updater, value Value) int64
	Target(u *updater.Updater, value Value)
}

var methodRegister = map[int32]Method{}

func SetMethod(key int32, fun MethodValue) {
	methodRegister[key] = &defaultMethodValue{fun: fun}
}

func SetMethodHandle(key int32, handle Method) {
	methodRegister[key] = handle
}

func GetMethod(key int32) Method {
	return methodRegister[key]
}

type MethodValue func(u *updater.Updater, value Value) int64

type defaultMethodValue struct {
	fun MethodValue
}

func (this *defaultMethodValue) Value(u *updater.Updater, value Value) int64 {
	return this.fun(u, value)
}
func (this *defaultMethodValue) Target(u *updater.Updater, value Value) {}
