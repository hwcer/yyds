package itypes

import (
	"github.com/hwcer/updater"
	"server/config"
	"server/define"
)

var ITypeHero = newItemsIType(define.ITypeHero)

func init() {
	ITypeHero.SetStacked(true)
	//ITypeHero.SetAttach(itemsHeroAttach)
	//ITypeHero.SetResolve(itemsHeroResolve)
}

func itemsHeroAttach(u *updater.Updater, item *Item) (r any, err error) {
	return
}

// Resolve 自动分解
func itemsHeroResolve(u *updater.Updater, iid int32, val int64) error {
	if c := config.Data.Hero[iid]; c != nil && c.Soul > 0 {
		//if q := config.Data.HeroQuality[c.Quality]; q != nil && q.CombineSoul > 0 {
		//	u.Add(c.Soul, int32(val)*q.CombineSoul)
		//}
	}
	return nil
}
