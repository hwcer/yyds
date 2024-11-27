package model

import (
	"github.com/hwcer/cosgo/random"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/operator"
	"server/config"
	"server/define"
)

var ITypeItemGroup = &Packs{IType: IType{id: define.ITypeItemGroup}}
var ITypeItemPacks = &Packs{IType: IType{id: define.ITypeItemPacks}}

//func init() {
//	im := &Packs{}
//	types := []updater.IType{ITypeItemGroup, ITypeItemPacks}
//	if err := updater.Register(updater.ParserTypeValues, updater.RAMTypeAlways, im, types...); err != nil {
//		logger.Panic(err)
//	}
//}

type Packs struct {
	IType
}

// Listener 独立概率
func (this *Packs) Listener(u *updater.Updater, op *operator.Operator) {
	if op.Type != operator.TypesAdd || op.Value <= 0 {
		return
	}
	op.Type = operator.TypesResolve
	r := map[int32]int32{}
	this.ParseItemProbability(op.IID, int32(op.Value), r)
	for k, v := range r {
		u.Add(k, v)
	}
}

// ParseItemProbability 解析概率表 物品组或者包
func (this *Packs) ParseItemProbability(k, v int32, r map[int32]int32) {
	this.SwitchItemParse(k, v, r)
}

// ParseItemPacks TODO 防止环形调用
func (this *Packs) ParseItemPacks(k, v int32, r map[int32]int32) {
	rows := config.Data.GetItemPacks(k)
	if rows == nil {
		logger.Debug("itemPacks not exist:%v", k)
		return
	}
	for i := 0; i < int(v); i++ {
		for _, row := range rows {
			if random.Probability(row.GetVal()) {
				this.SwitchItemParse(row.Key, row.Num, r)
			}
		}
	}
}

func (this *Packs) ParseItemGroup(k, v int32, r map[int32]int32) {
	w := config.Data.GetItemGroup(k)
	if w == nil {
		logger.Debug("itemPacks not exist:%v", k)
		return
	}
	for i := 0; i < int(v); i++ {
		if x := w.Roll(); x >= 0 {
			c := config.Data.ItemGroup[x]
			this.SwitchItemParse(c.Key, c.Num, r)
		}
	}
}

func (this *Packs) SwitchItemParse(k, v int32, r map[int32]int32) {
	switch config.Data.GetIType(k) {
	case define.ITypeItemPacks:
		this.ParseItemPacks(k, v, r)
	case define.ITypeItemGroup:
		this.ParseItemGroup(k, v, r)
	default:
		r[k] += v
	}
}
