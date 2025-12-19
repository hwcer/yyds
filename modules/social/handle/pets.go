package handle

import (
	"server/game/cache"
	"server/game/model"
	"server/game/response"
	"server/share/config"

	"github.com/hwcer/cosgo/registry"

	"github.com/hwcer/yyds"
	"github.com/hwcer/yyds/context"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

func init() {
	Register(&pets{})
}

type pets struct {
}

func (this *pets) Caller(node *registry.Node, handle *context.Context) interface{} {
	f := node.Method().(func(*pets, *context.Context) interface{})
	return f(this, handle)
}

// Collect 偷钱
// 对应路由: POST /social/pets/Collect
func (b *pets) Collect(ctx *context.Context) any {
	args := struct {
		Fid   string `json:"fid"`    //好友ID
		PetID string `json:"petId" ` // 蛋ID（UUID）

	}{}
	if err := ctx.Bind(&args); err != nil {
		return err
	}
	if args.Fid == "" || args.PetID == "" {
		return errors.ErrArgEmpty
	}

	m := Home.CollectMaxNum(ctx.Player, args.Fid)
	n := Home.LogsNum(ctx.Player, args.Fid, model.HomeLogsTypeGold)
	if n >= m {
		return ctx.Error("次数不足")
	}

	n = Home.LogsNum(ctx.Player, args.Fid, model.HomeLogsTypeGold, args.PetID)
	if n >= 1 {
		return ctx.Error("这只今天偷过了，换一只吧")
	}

	//if _, err := GetPetsCollect(ctx.Uid(), args.Fid, args.PetID); err != nil {
	//	return err
	//}
	//
	//v := ctx.Player.Val(config.Data.DailyKey.FriendCollectGold)
	//if v >= int64(config.Data.Base.FriendCollectGold) {
	//	return ctx.Error("可用次数不足")
	//}
	//now := ctx.Unix()
	////好友检查
	//err := socialModel.Graph.Modify(ctx.Uid(), func(p *graph.Player) error {
	//	gp := p.Friend(args.Fid)
	//	if gp == nil {
	//		return ctx.Error("对方不是你的好友")
	//	}
	//	var list map[string]int64
	//	s := gp.Get(socialModel.PlayerValuesKeyCollectGold)
	//	if s != nil {
	//		list, _ = s.(map[string]int64)
	//	} else {
	//		list = make(map[string]int64)
	//	}
	//	if r, ok := list[args.PetID]; ok && now-r < 3600 {
	//		return ctx.Error("请不要连续薅同一只宠物")
	//	}
	//	list[args.PetID] = now
	//	gp.Set(socialModel.PlayerValuesKeyCollectGold, list)
	//	return nil
	//})
	//if err != nil {
	//	return err
	//}

	var gold int64
	var goldPerSecond float64
	now := ctx.Unix()
	resPet := &response.Pet{}

	err := ctx.GetPlayer(args.Fid, true, func(p *player.Player) error {
		roleMod := p.Document(config.ITypeRole).Any().(*model.Role)
		if now-roleMod.Login > config.Data.Base.FriendCollectOfflineLimie*3600 {
			return ctx.Errorf(1003, "对方太久没登录啦，喊他上线才能偷")
		}

		items := cache.GetItems(p.Updater)
		pet := items.Get(args.PetID)
		if pet == nil {
			return ctx.Error("宠物不存在")
		}
		if yyds.Config.GetIType(pet.IID) != config.ITypePet {
			return ctx.Error("不是宠物")
		}
		x := float64(1)
		//if p.Status != player.StatusConnected {
		//	x = 100
		//}
		gold, goldPerSecond = pet.GetPetCalculateFinalCollectableGold(p, x, int64(config.Data.Base.StealTimeLimit))
		pr := cache.GetRole(p.Updater)
		pr.Add("likes", 1)
		_, _ = p.Submit()
		resPet.Convert(p, pet, items.GetPlot(pet))
		return nil
	})
	if err != nil {
		return err
	}
	if v := int64(goldPerSecond * 24 * 3600); gold > v {
		gold = v
	}
	gold = gold * CollectGoldRatio / 100
	ctx.Player.Add(config.Data.RoleFields.Gold, gold)
	//ctx.Player.Add(config.Data.DailyKey.FriendCollectGold, 1)
	roleMod := ctx.Player.Document(config.ITypeRole).Any().(*model.Role)

	logs := model.HomeLogs{}
	logs.Uid = args.Fid
	logs.FID = ctx.Uid()
	logs.FName = roleMod.Name
	logs.IType = model.HomeLogsTypeGold
	logs.Target = args.PetID
	logs.Create = ctx.Unix()
	logs.ID = model.ObjectId.Simple()

	if err = model.DB().Create(&logs).Error; err != nil {
		return err
	}
	reply := map[string]interface{}{
		"pet":       resPet,
		"gold":      gold,
		"TotalGold": gold + roleMod.Gold,
	}
	return reply

}
