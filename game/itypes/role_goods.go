package model

import (
	"errors"
	"fmt"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"server/define"
)

const roleGoodsField = "goods"
const roleGoodsFormat = "goods.%v"

func init() {
	im := &roleGoods{}
	it := NewIType(define.ITypeGoods)
	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, im, it); err != nil {
		logger.Panic(err)
	}
}

type roleGoods struct {
}

func (this *roleGoods) Getter(u *updater.Updater, values *dataset.Values, keys []int32) error {
	//内存模式只会拉所有
	if len(keys) > 0 {
		return errors.New("record getter 参数keys应该为空")
	}
	role := u.Handle(define.ITypeRole).(*updater.Document).Any().(*Role)
	if role.Goods == nil {
		values.Reset(make(map[int32]int64), 0)
	} else {
		values.Reset(role.Goods, 0)
	}
	return nil
}

func (this *roleGoods) Setter(u *updater.Updater, values dataset.Data, expire int64) error {
	doc := u.Handle(define.ITypeRole).(*updater.Document)
	role := doc.Any().(*Role)
	if len(role.Goods) == 0 {
		data := u.Handle(define.ITypeGoods).(*updater.Values).All()
		role.Goods = data
		doc.Dirty(roleGoodsField, data)
		return nil
	}
	for k, v := range values {
		field := fmt.Sprintf(roleGoodsFormat, k)
		doc.Dirty(field, v)
	}
	return nil
}
