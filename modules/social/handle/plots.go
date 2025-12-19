package handle

import (
	"server/game/cache"
	"server/game/itypes"
	"server/game/model"
	"server/game/response"
	"server/share/config"

	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/yyds/context"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

func init() {
	Register(&Plots{})
}

type Plots struct {
}

func (this *Plots) Caller(node *registry.Node, handle *context.Context) interface{} {
	f := node.Method().(func(*Plots, *context.Context) interface{})
	return f(this, handle)
}

// Unlock 金币解锁格子
// 对应路由: POST /social/plots/Unlock
func (b *Plots) Unlock(ctx *context.Context) any {
	args := struct {
		Fid    string `json:"fid"`     //好友ID
		PlotID int32  `json:"plotId" ` // 地块
	}{}
	if err := ctx.Bind(&args); err != nil {
		return err
	}
	if args.Fid == "" || args.PlotID <= 0 {
		return errors.ErrArgEmpty
	}
	plotConfig := config.Data.Land[args.PlotID]
	if plotConfig == nil {
		return ctx.Error("地块不存在")
	}
	if plotConfig.Unlock != 0 && plotConfig.Unlock != 1 {
		return ctx.Error("只能帮忙解锁金币格子")
	}
	if n := Home.LogsNum(ctx.Player, args.Fid, model.HomeLogsTypeUnlockPlot); n >= int64(config.Data.Base.FriendUnlockPlot) {
		return ctx.Error("次数不足")
	}

	roleDoc := cache.GetRole(ctx.Player.Updater)
	roleMod := roleDoc.All()
	if plotConfig.Price > 0 && roleMod.Gold < plotConfig.Price {
		return ctx.Error("金币不足，无法解锁")
	}
	Price := plotConfig.Price
	resPlot := &response.Plot{}
	var PlotId string
	err := ctx.GetPlayer(args.Fid, true, func(p *player.Player) error {
		items := cache.GetItems(p.Updater)
		pet := items.Get(args.PlotID)
		if pet != nil {
			return ctx.Errorf(1100, "格子已经解锁")
		}
		item, err := itypes.Land.Create(p.Updater, plotConfig.Id, 1)
		if err != nil {
			return err
		}
		_ = p.Collection(config.ITypeLand).New(item)

		pr := cache.GetRole(p.Updater)
		pr.Add("likes", 1)
		_, _ = p.Submit()
		PlotId = item.OID
		resPlot.Convert(p, item, nil)
		return nil
	})

	if err != nil {
		return err
	}

	if Price > 0 {
		ctx.Player.Sub(config.Data.RoleFields.Gold, Price)
	}

	logs := model.HomeLogs{}
	logs.Uid = args.Fid
	logs.FID = ctx.Uid()
	logs.FName = roleMod.Name
	logs.IType = model.HomeLogsTypeUnlockPlot
	logs.Target = PlotId
	logs.Create = ctx.Unix()

	logs.ID = model.ObjectId.Simple()
	_ = model.DB().Create(&logs)

	reply := map[string]interface{}{
		"plot":      resPlot,
		"gold":      Price,
		"TotalGold": roleMod.Gold - Price,
	}
	return reply
}
