package updater

import (
	"github.com/hwcer/cosgo/values"
)

var (
	ErrCodeArgsIllegal   = 0
	ErrCodeItemNotExist  = 0
	ErrCodeItemNotEnough = 0
	ErrCodeITypeNotExist = 0
	ErrCodeObjectIdEmpty = 0
)

func Errorf(code int, msg any, args ...any) error {
	return values.Errorf(code, msg, args...)
}

func ErrArgsIllegal(args ...any) error {
	return Errorf(ErrCodeArgsIllegal, "args illegal:%v", args)
}

func ErrItemNotExist(id any) error {
	return Errorf(ErrCodeItemNotExist, "Item Not Exist:%v", id)
}

func ErrItemNotEnough(args ...any) error {
	return Errorf(ErrCodeItemNotEnough, "Item Not Enough:%v", args)
}

func ErrITypeNotExist(iid int32) error {
	return Errorf(ErrCodeITypeNotExist, "IType Not Exist%v", iid)
}

func ErrObjectIdEmpty(args ...any) error {
	return Errorf(ErrCodeObjectIdEmpty, "oid empty:%v", args)
}

var (
	ErrUnableUseIIDOperation = Errorf(0, "unable to use iid operation")
	ErrSubmitEndlessLoop     = Errorf(0, "submit endless loop") //出现死循环,检查事件和插件是否正确移除(返回false)
)
