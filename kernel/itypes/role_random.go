package itypes

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
	Random func(iid, num int32) map[int32]int32
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
	r := map[int32]int32{}
	this.switchParser(op.IID, int32(op.Value), r, 0)
	for k, v := range r {
		u.Add(k, v)
	}
}

// parse 解析概率表 物品组或者包
func (this *itemsRandom) parser(k, v int32, r map[int32]int32, n int32) {
	for i, j := range this.Random(k, v) {
		this.switchParser(i, j, r, n)
	}
}

func (this *itemsRandom) switchParser(k, v int32, r map[int32]int32, n int32) {
	n++
	if n > 100 {
		logger.Alert("IType itemsRandom endless loop:%v", k)
	}
	switch config.GetIType(k) {
	case config.ITypeItemPacks:
		ItemsPacks.parser(k, v, r, n)
	case config.ITypeItemGroup:
		ItemsGroup.parser(k, v, r, n)
	default:
		r[k] += v
	}
}
