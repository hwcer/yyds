package itype

import (
	"github.com/hwcer/yyds/kernel/config"
)

var Gacha = NewItemsIType(config.ITypeGacha)

func init() {
	Gacha.SetStacked(true)
}

const (
	GachaAttachLess = "less" //累计出现保底消耗的次数
	GachaAttachSpec = "spec" //累计出现保底次数
	GachaAttachWish = "wish" //许愿池 GachaRate -> GachaGroup  map[int32]int32
)
