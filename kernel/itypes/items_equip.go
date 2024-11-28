package itypes

import (
	"github.com/hwcer/yyds/kernel/config"
)

var Equip = NewItemsIType(config.ITypeEquip)

func init() {
	Equip.SetStacked(false)
	//ITypeEquip.SetAttach(itemsEquipAttach)
}

//func itemsEquipAttach(u *updater.Updater, item *Item) (r any, err error) {
//	return
//}
