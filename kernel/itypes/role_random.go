package itype

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/operator"
	"github.com/hwcer/yyds/kernel/config"
)

var ItemsGroup = &itemsRandom{IType: IType{id: config.ITypeItemGroup}}
var ItemsPacks = &itemsRandom{IType: IType{id: config.ITypeItemPacks}}

type itemsRandom struct {
	IType
	Random func(u *updater.Updater, iid, num int32) map[int32]int32
}

// Listener 独立概率
func (this *itemsRandom) Listener(u *updater.Updater, op *operator.Operator) {
	if this.Random == nil {
		logger.Alert("请配置物品组相关处理方法")
		return
	}
	if op.Type != operator.TypesAdd || op.Value <= 0 {
		return
	}
	op.Type = operator.TypesResolve
	r := this.Parse(u, op.IID, int32(op.Value))
	for k, v := range r {
		u.Add(k, v)
	}
}

func (this *itemsRandom) Parse(u *updater.Updater, iid, num int32) map[int32]int32 {
	r := map[int32]int32{}
	this.switchParser(u, iid, num, r, 0)
	return r
}

// parse 解析概率表 物品组或者包
func (this *itemsRandom) doRandom(u *updater.Updater, k, v int32, r map[int32]int32, n int32) {
	if this.Random == nil {
		logger.Alert("itemsRandom Random handle is nil")
		return
	}
	rs := this.Random(u, k, v)
	for i, j := range rs {
		this.switchParser(u, i, j, r, n)
	}
}

func (this *itemsRandom) switchParser(u *updater.Updater, k, v int32, r map[int32]int32, n int32) {
	n++
	if n > 100 {
		logger.Alert("IType itemsRandom endless loop:%v", k)
	}
	switch config.GetIType(k) {
	case config.ITypeItemPacks:
		ItemsPacks.doRandom(u, k, v, r, n)
	case config.ITypeItemGroup:
		ItemsGroup.doRandom(u, k, v, r, n)
	default:
		r[k] += v
	}
}
