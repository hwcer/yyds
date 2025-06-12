package errors

import (
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/logger"
)

var errorDict = make(map[int32]*values.Message)

func Register(msg ...*values.Message) {
	for _, m := range msg {
		if _, ok := errorDict[m.Code]; ok {
			logger.Fatal("错误码重复：%v", m.Code)
		} else {
			errorDict[m.Code] = m
		}
	}
}
