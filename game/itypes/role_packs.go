package itypes

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/random"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/operator"
	"github.com/hwcer/yyds/game/config"
	"github.com/hwcer/yyds/game/share"
)

var ItemsGroup = &Packs{IType: IType{id: share.ITypeItemGroup}}
var ItemsPacks = &Packs{IType: IType{id: share.ITypeItemPacks}}

type Packs struct {
	IType
}

// Listener 独立概率
func (this *Packs) Listener(u *updater.Updater, op *operator.Operator) {
	if Options.GetItemsPacksConfig == nil || Options.GetItemsGroupConfig == nil || Options.GetItemsGroupRandom == nil {
		logger.Alert("请配置物品组相关处理方法")
	}
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
	rows := Options.GetItemsPacksConfig(k)
	if rows == nil {
		logger.Debug("itemPacks not exist:%v", k)
		return
	}
	for i := 0; i < int(v); i++ {
		for _, row := range rows {
			if random.Probability(row.GetVal()) {
				this.SwitchItemParse(row.GetKey(), row.GetNum(), r)
			}
		}
	}
}

func (this *Packs) ParseItemGroup(k, v int32, r map[int32]int32) {
	w := Options.GetItemsGroupRandom(k)
	if w == nil {
		logger.Debug("itemPacks not exist:%v", k)
		return
	}
	for i := 0; i < int(v); i++ {
		if x := w.Roll(); x >= 0 {
			c := Options.GetItemsGroupConfig(x)
			this.SwitchItemParse(c.GetKey(), c.GetNum(), r)
		}
	}
}

func (this *Packs) SwitchItemParse(k, v int32, r map[int32]int32) {
	switch config.GetIType(k) {
	case share.ITypeItemPacks:
		this.ParseItemPacks(k, v, r)
	case share.ITypeItemGroup:
		this.ParseItemGroup(k, v, r)
	default:
		r[k] += v
	}
}
