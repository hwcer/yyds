package itypes

import "github.com/hwcer/yyds/game/share"

var Equip = NewItemsIType(share.ITypeEquip)

func init() {
	Equip.SetStacked(false)
	//ITypeEquip.SetAttach(itemsEquipAttach)
}

//func itemsEquipAttach(u *updater.Updater, item *Item) (r any, err error) {
//	return
//}
