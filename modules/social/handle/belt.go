package handle

import (
	"server/game/analytics"
	"server/game/cache"

	"server/game/itypes"
	"server/game/model"
	"server/game/response"
	"server/share/config"

	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/uuid"

	"github.com/hwcer/yyds/context"
	"github.com/hwcer/yyds/errors"
)

func init() {
	Register(&belt{})
}

type belt struct {
}

func (this *belt) Caller(node *registry.Node, handle *context.Context) interface{} {
	f := node.Method().(func(*belt, *context.Context) interface{})
	return f(this, handle)
}

// Create 生成一批蛋
func (b *belt) Create(ctx *context.Context) any {
	args := struct {
		Fid string `json:"fid"` //好友ID
	}{}
	if err := ctx.Bind(&args); err != nil {
		return err
	}
	if args.Fid == "" {
		return errors.ErrArgEmpty
	}
	role := &model.Role{}
	tx := model.DB().Select("beltUsed", "belt").Where(args.Fid).First(role)
	if tx.Error != nil {
		return tx.Error
	} else if tx.RowsAffected == 0 {
		return ctx.Error("好友不存在")
	}
	beltUsedType := role.Belt
	if role.BeltUsed > 0 {
		beltUsedType = role.BeltUsed
	}
	var selectedEggs []*response.BeltEgg
	// 根据传送带类型配置生成新蛋
	for range 20 {
		if k, m := config.RandomBeltEgg(beltUsedType); k > 0 {
			prefix := uint64(k)<<32 + uint64(m)
			o := model.ObjectId.New(prefix)
			egg := model.BeltEgg{ID: o, EggType: k, Mutation: m}
			resBeltEgg := &response.BeltEgg{}
			resBeltEgg.Convert(ctx.Player, o, &egg)
			selectedEggs = append(selectedEggs, resBeltEgg)
		}
	}
	reply := map[string]interface{}{}
	reply["beltEggs"] = selectedEggs
	return reply
}

// egg 从好友传送带选择蛋
// 对应路由: POST /social/belt/egg
func (b *belt) Egg(ctx *context.Context) any {
	args := struct {
		Fid   string `json:"fid"`    //好友ID
		EggID string `json:"eggId" ` // 蛋ID（UUID）
		//PlotX int32  `json:"plotX" ` // 地块X坐标
		//PlotY int32  `json:"plotY" ` // 地块Y坐标
	}{}
	if err := ctx.Bind(&args); err != nil {
		return err
	}
	if args.Fid == "" || args.EggID == "" {
		return errors.ErrArgEmpty
	}
	//检查是否好友，忽略
	key, _, err := uuid.Split(args.EggID, uuid.BaseSize, 1)
	if err != nil {
		return err
	}
	iid := int32(key >> 32)
	mutation := int32(key & 0xffffffff)
	//保存蛋
	egg, err := itypes.Egg.CreateEgg(ctx.Player.Updater, iid, mutation)
	if err != nil {
		return err
	}
	items := cache.GetItems(ctx.Player.Updater)
	if err = items.New(egg); err != nil {
		return err
	}
	roleDoc := cache.GetRole(ctx.Player.Updater)
	roleMod := roleDoc.All()
	resEgg := &response.Egg{}
	resEgg.Convert(ctx.Player, egg, nil)
	//扣钱
	price := resEgg.GetPrice()
	ctx.Player.Sub(config.Data.RoleFields.Gold, price)

	// 上报买蛋事件
	analytics.TrackBuyEgg(ctx, egg.OID, iid, mutation, price)

	userInfo := struct {
		//BeltEggs []*response.BeltEgg `json:"beltEggs" bson:"beltEggs"` // 传送带上的蛋信息数组
		Gold int64 `json:"gold" db:"gold"` //金币
	}{
		Gold: roleMod.Gold - price,
	}
	//for k, v := range newBeltEggs {
	//	resBelt := &response.BeltEgg{}
	//	resBelt.Convert(ctx.Player, k, &v)
	//	userInfo.BeltEggs = append(userInfo.BeltEggs, resBelt)
	//}

	res := struct {
		EggID string        `json:"eggId"` // 选择的蛋ID
		Egg   *response.Egg `json:"egg"`   // 当前蛋的最新数据
		PlotX int32         `json:"plotX"` // 地块X坐标
		PlotY int32         `json:"plotY"` // 地块Y坐标
		User  any           `json:"user"`  // 用户信息
		Likes int64         `json:"likes"`
	}{
		EggID: egg.OID,
		Egg:   resEgg,
		PlotX: resEgg.PlotX,
		PlotY: resEgg.PlotY,
		User:  userInfo,
		Likes: 1,
	}
	ctx.Next = func() {
		AddLikes(ctx, args.Fid)
	}
	return res
}

// gift 从好友传送带上赠送蛋接口
// 对应路由: POST /social/belt/gift
func (b *belt) Gift(ctx *context.Context) any {
	args := struct {
		Fid   string `json:"fid"`    //好友ID
		EggID string `json:"eggId" ` // 蛋ID（UUID）
		//PlotX int32  `json:"plotX" ` // 地块X坐标
		//PlotY int32  `json:"plotY" ` // 地块Y坐标
	}{}
	if err := ctx.Bind(&args); err != nil {
		return err
	}
	if args.Fid == "" || args.EggID == "" {
		return errors.ErrArgEmpty
	}
	//检查是否好友，忽略
	key, _, err := uuid.Split(args.EggID, uuid.BaseSize, 1)
	if err != nil {
		return err
	}
	eggType := int32(key >> 32)
	mutation := int32(key & 0xffffffff)

	//从传送带上拿蛋要付钱的
	item, _ := itypes.Egg.CreateEgg(ctx.Player.Updater, eggType, mutation)
	resEgg := response.NewEgg(item.OID, eggType, mutation)

	price := resEgg.GetPrice()
	ctx.Player.Sub(config.Data.RoleFields.Gold, price)

	gift := &model.Gift{
		Iid:      eggType,
		Mutation: mutation,
		Sender:   ctx.Uid(),
		Receiver: "",
		Status:   0,
		Expire:   ctx.Unix() + 7*24*60*60,
		Created:  ctx.Unix(),
	}
	gift.ID = model.ObjectId.Simple()

	if err = model.DB().Create(gift).Error; err != nil {
		return err
	}
	ctx.Next = func() {
		AddLikes(ctx, args.Fid)
	}
	return response.NewGiftEgg(ctx.Player, gift, true)
}
