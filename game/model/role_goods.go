package model

import (
	"errors"
	"fmt"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/yyds/game/share"
)

const roleGoodsField = "goods"
const roleGoodsFormat = "goods.%v"

type Goods struct {
}

func (this *Goods) Getter(u *updater.Updater, values *dataset.Values, keys []int32) error {
	//内存模式只会拉所有
	if len(keys) > 0 {
		return errors.New("record getter 参数keys应该为空")
	}
	doc := u.Handle(share.ITypeRole).(*updater.Document)

	if i := doc.Get(roleGoodsField); i == nil {
		values.Reset(make(map[int32]int64), 0)
	} else {
		values.Reset(i.(map[int32]int64), 0)
	}
	return nil
}

func (this *Goods) Setter(u *updater.Updater, values dataset.Data, expire int64) error {
	doc := u.Handle(share.ITypeRole).(*updater.Document)
	var goods map[int32]int64
	if i := doc.Get(roleGoodsField); i != nil {
		goods = i.(map[int32]int64)
	}
	if len(goods) == 0 {
		data := u.Handle(share.ITypeGoods).(*updater.Values).All()
		doc.Dirty(roleGoodsField, data)
		return nil
	}
	for k, v := range values {
		field := fmt.Sprintf(roleGoodsFormat, k)
		doc.Dirty(field, v)
	}
	return nil
}
