package model

import (
	"errors"
	"fmt"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"server/define"
)

const roleRecordField = "record"
const roleRecordFormat = "record.%v"

func init() {
	im := &roleRecord{}
	it := NewIType(define.ITypeRecord)
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, im, it); err != nil {
		logger.Panic(err)
	}
}

type roleRecord struct {
}

func (this *roleRecord) Getter(u *updater.Updater, values *dataset.Values, keys []int32) error {
	//内存模式只会拉所有
	if len(keys) > 0 {
		return errors.New("record getter 参数keys应该为空")
	}
	role := u.Handle(define.ITypeRole).(*updater.Document).Any().(*Role)
	if role.Record == nil {
		values.Reset(make(map[int32]int64), 0)
	} else {
		values.Reset(role.Record, 0)
	}
	return nil
}

func (this *roleRecord) Setter(u *updater.Updater, values dataset.Data, expire int64) error {
	doc := u.Handle(define.ITypeRole).(*updater.Document)
	role := doc.Any().(*Role)
	if len(role.Record) == 0 {
		var data map[int32]int64
		data = u.Handle(define.ITypeRecord).(*updater.Values).All()
		role.Record = data
		doc.Dirty(roleRecordField, data)
		return nil
	}
	for k, v := range values {
		field := fmt.Sprintf(roleRecordFormat, k)
		doc.Dirty(field, v)
	}
	return nil
}
