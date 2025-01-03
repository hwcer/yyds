package errors

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/values"
)

var errorDict = make(map[int]*values.Message)

func Register(msg ...*values.Message) {
	for _, m := range msg {
		if _, ok := errorDict[m.Code]; ok {
			logger.Fatal("错误码重复：%v", m.Code)
		} else {
			errorDict[m.Code] = m
		}
	}
}
