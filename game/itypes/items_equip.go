package itypes

import (
	"server/define"
)

var ITypeEquip = newItemsIType(define.ITypeEquip)

func init() {
	ITypeEquip.SetStacked(false)
	//ITypeEquip.SetAttach(itemsEquipAttach)
}

//func itemsEquipAttach(u *updater.Updater, item *Item) (r any, err error) {
//	return
//}
