package handle

import (
	"server/game/cache"
	socialModel "server/game/handle/social/model"
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
	Register(&eggs{})
}

type eggs struct {
}

func (this *eggs) Caller(node *registry.Node, handle *context.Context) interface{} {
	f := node.Method().(func(*eggs, *context.Context) interface{})
	return f(this, handle)
}

// Fast 加速蛋
// 对应路由: POST /social/pets/Fast
func (b *eggs) Fast(ctx *context.Context) any {
	args := struct {
		Fid   string `json:"fid"`    //好友ID
		EggID string `json:"eggId" ` // 蛋ID（UUID）
	}{}
	if err := ctx.Bind(&args); err != nil {
		return err
	}
	if args.Fid == "" || args.EggID == "" {
		return errors.ErrArgEmpty
	}
	v := Home.LogsNum(ctx.Player, args.Fid, model.HomeLogsTypeFastEgg)
	//v := ctx.Player.Val(config.Data.DailyKey.FriendFastEgg)
	if v >= int64(config.Data.Base.FriendFastEgg) {
		return ctx.Error("可用次数不足")
	}
	now := ctx.Unix()
	if !socialModel.Graph.Has(ctx.Uid(), args.Fid) {
		return ctx.Error("不是好友")
	}

	res := &response.Egg{}

	err := ctx.GetPlayer(args.Fid, true, func(p *player.Player) error {
		items := cache.GetItems(p.Updater)
		egg := items.Get(args.EggID)
		if egg == nil {
			return ctx.Error("蛋不存在")
		}
		if yyds.Config.GetIType(egg.IID) != config.ITypeEgg {
			return ctx.Error("不是蛋")
		}
		t := egg.Attach.GetInt64(model.AttachTypeTimes)
		t -= 1800
		items.SetAttach(egg.OID, model.AttachTypeTimes, t)
		pr := cache.GetRole(p.Updater)
		pr.Add("likes", 1)
		_, _ = p.Submit()
		res.Convert(p, egg, nil)
		return nil
	})
	if err != nil {
		return err
	}

	//ctx.Player.Add(config.Data.DailyKey.FriendFastEgg, 1)
	roleMod := ctx.Player.Document(config.ITypeRole).Any().(*model.Role)

	logs := model.HomeLogs{}
	logs.Uid = args.Fid
	logs.FID = ctx.Uid()
	logs.FName = roleMod.Name
	logs.IType = model.HomeLogsTypeFastEgg
	logs.Create = now
	logs.Target = args.EggID
	logs.ID = model.ObjectId.Simple()

	_ = model.DB().Create(&logs)

	reply := map[string]interface{}{}
	reply["egg"] = res
	return reply

}
