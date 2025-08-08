package errors

import (
	"errors"

	"github.com/hwcer/cosgo/values"
)

var Is = errors.Is
var As = errors.As
var Join = errors.Join
var Unwrap = errors.Unwrap

func New(err any) error {
	return values.Error(err)
}

func Error(err any) error {
	return values.Error(err)
}

func Errorf(code int32, format any, args ...any) error {
	return values.Errorf(code, format, args...)
}
