package handle

import (
	"server/game/model"
	"server/game/response"
	"server/share"
	"server/share/config"

	sm "server/game/handle/social/model"

	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/yyds/context"
	"github.com/hwcer/yyds/players/player"
)

var Home = &home{}

func init() {
	Register(Home)
}

const (
	//CollectVideoMax  = 10 //每日最大次数
	//CollectVideRatio = 3  //每次加偷菜次数
	CollectGoldRatio = 10 //偷取10%
)

type home struct {
}

func (this *home) Caller(node *registry.Node, handle *context.Context) interface{} {
	f := node.Method().(func(*home, *context.Context) interface{})
	return f(this, handle)
}

type HomeLaunchRole struct {
	share.Player `bson:"inline"`
	Belt         int32   `json:"belt" bson:"belt"`
	BeltUsed     int32   `json:"beltUsed" bson:"beltUsed"` // 用户当前使用的传送带等级，为0时表示使用最大等级
	Mutation     []int32 `json:"mutation" bson:"mutation"`
}
type HomeVisitReply struct {
	User                  HomeLaunchRole
	Plots                 map[string]*response.Plot `json:"plots"` //新增
	BeltEggs              []*response.BeltEgg       `json:"beltEggs"`
	Eggs                  map[string]*response.Egg  `json:"eggs"`
	Pets                  map[string]*response.Pet  `json:"pets"`
	CollectGoldCur        int64                     `json:"collectGoldCur"`        //今日已偷
	CollectGoldMax        int64                     `json:"collectGoldMax"`        //今日最大 包含看广告次数
	VideoCollectRemaining int64                     `json:"videoCollectRemaining"` //剩余广告次数
	VideoFastEggRemaining int64                     `json:"videoFastEggRemaining"` //剩余加速次数
	CollectGoldRatio      int64                     `json:"collectGoldRatio"`      //偷取系数  10  (10%)
	CollectVideRatio      int64                     `json:"collectVideRatio"`      //每次看视频加N次偷菜机会
}

// Visit 拜访好友家园
// home/Visit
func (this *home) Visit(c *context.Context) any {
	fid := c.GetString("fid")
	if fid == "" {
		return c.Errorf(0, "fid is empty")
	}
	if c.Uid() == fid {
		return c.Errorf(0, "fid is empty")
	}

	var reply *HomeVisitReply
	err := c.GetPlayer(fid, true, func(player *player.Player) error {
		r, e := this.Launch(player)
		if e != nil {
			return e
		}
		reply = r
		return nil
	})
	if err != nil {
		return err
	}
	if reply != nil {
		collectVideoNum := this.LogsNum(c.Player, fid, model.HomeLogsTypeVideoCollect)
		videoRemaining := int64(config.Data.Base.VideoCollectRemaining) - collectVideoNum
		if videoRemaining <= 0 {
			videoRemaining = 0
		}
		reply.VideoCollectRemaining = videoRemaining
		//int64(config.Data.Base.FriendCollectGold) + collectVideoNum*CollectVideRatio
		//reply.CollectGoldMax = int64(config.Data.Base.FriendCollectGold)
		CollectGoldMax := int64(config.Data.Base.FriendCollectGold) + collectVideoNum*int64(config.Data.Base.CollectVideRatio)
		reply.CollectGoldMax = CollectGoldMax

		reply.CollectGoldCur = this.LogsNum(c.Player, fid, model.HomeLogsTypeGold)
		if reply.CollectGoldCur > CollectGoldMax {
			reply.CollectGoldCur = CollectGoldMax
		}

		reply.VideoFastEggRemaining = int64(config.Data.Base.FriendFastEgg) - this.LogsNum(c.Player, fid, model.HomeLogsTypeFastEgg)
		if reply.VideoFastEggRemaining < 0 {
			reply.VideoFastEggRemaining = 0
		}
		reply.CollectGoldRatio = CollectGoldRatio
		reply.CollectVideRatio = int64(config.Data.Base.CollectVideRatio)
	}

	return reply
}

func (this *home) Logs(c *context.Context) any {
	paging := &cosmo.Paging{}
	if err := c.Bind(paging); err != nil {
		return err
	}
	paging.Init(100)
	var rows []*model.HomeLogs
	paging.Rows = &rows
	tx := model.DB().Model(&model.HomeLogs{}).Where("uid = ?", c.Uid()).Order("create", -1).Page(paging)
	if tx.Error != nil {
		return tx.Error
	}
	return paging

}

// 看广告加次数
// 每天10 次 ，每次3
func (this *home) Video(c *context.Context) any {
	fid := c.GetString("fid")
	if fid == "" {
		return c.Errorf(0, "fid is empty")
	}
	if !sm.Graph.Has(c.Uid(), fid) {
		return c.Errorf(0, "fid is not exist")
	}
	n := this.LogsNum(c.Player, fid, model.HomeLogsTypeVideoCollect)
	if n >= int64(config.Data.Base.VideoCollectRemaining) {
		return c.Errorf(0, "次数不足")
	}

	log := &model.HomeLogs{}
	log.ID = model.ObjectId.Simple()
	log.Uid = fid
	log.FID = c.Uid()
	log.IType = model.HomeLogsTypeVideoCollect
	log.Create = c.Unix()

	if tx := model.DB().Create(log); tx.Error != nil {
		return tx.Error
	}
	reply := map[string]any{}
	reply["num"] = n + 1
	reply["max"] = 10
	reply["CollectGoldMax"] = config.Data.Base.FriendCollectGold + int32(n+1)*config.Data.Base.CollectVideRatio
	reply["CollectGoldCur"] = this.LogsNum(c.Player, fid, model.HomeLogsTypeGold)
	return reply
}

func (this *home) CollectMaxNum(p *player.Player, fid string) int64 {
	collectVideoNum := this.LogsNum(p, fid, model.HomeLogsTypeGold)
	return int64(config.Data.Base.FriendCollectGold) + collectVideoNum*int64(config.Data.Base.CollectVideRatio)
}

func (this *home) LogsNum(p *player.Player, fid string, t model.HomeLogsType, target ...string) int64 {
	today := times.Daily(0).Now().Unix()
	tx := model.DB().Model(&model.HomeLogs{}).Where("fid = ?", p.Uid())
	tx = tx.Where("uid = ?", fid)
	tx = tx.Where("itype = ?", t)
	tx = tx.Where("create >= ?", today)
	tx = tx.Order("create", -1)

	if l := len(target); l == 1 {
		tx = tx.Where("target = ?", target[0])
	} else if l > 1 {
		tx = tx.Where("target IN ?", target)
	}

	var n int64
	if err := tx.Count(&n).Error; err != nil {
		return 0
	}

	return n
}

type HomeInfoReply struct {
	share.Player `bson:"inline"`
	Mutation     []int32 `json:"mutation" bson:"mutation"`
}

// home/Info 好友信息
func (this *home) Info(c *context.Context) any {
	fid := c.GetString("fid")
	if fid == "" {
		return c.Errorf(0, "fid is empty")
	}
	if c.Uid() == fid {
		return c.Errorf(0, "fid is empty")
	}
	reply := &HomeInfoReply{}
	err := c.GetPlayer(fid, true, func(player *player.Player) error {
		role := player.Document(config.ITypeRole).Any().(*model.Role)
		reply.Player = role.Player
		reply.Mutation = role.GetMutation()
		return nil
	})
	if err != nil {
		return err
	}
	return reply
}

func (this *home) Launch(p *player.Player) (*HomeVisitReply, error) {

	role := p.Document(config.ITypeRole).Any().(*model.Role)

	resLaunch, err := response.Launch(p)
	if err != nil {
		return nil, err
	}

	var selectedEggs []*response.BeltEgg
	beltUsedType := role.Belt
	if role.BeltUsed > 0 {
		beltUsedType = role.BeltUsed
	}
	// 根据传送带类型配置生成新蛋
	for range 20 {
		if k, m := config.RandomBeltEgg(beltUsedType); k > 0 {
			prefix := uint64(k)<<32 + uint64(m)
			o := model.ObjectId.New(prefix)
			egg := model.BeltEgg{ID: o, EggType: k, Mutation: m}
			resBeltEgg := &response.BeltEgg{}
			resBeltEgg.Convert(p, o, &egg)
			selectedEggs = append(selectedEggs, resBeltEgg)
		}
	}

	// 构建响应数据 - 注意排除已废弃的belt和beltConfigs字段
	res := &HomeVisitReply{
		User:     HomeLaunchRole{Player: role.Player, Belt: role.Belt, BeltUsed: role.BeltUsed, Mutation: role.GetMutation()},
		Plots:    resLaunch.Plots, //新增
		BeltEggs: selectedEggs,
		Eggs:     resLaunch.Eggs,
		Pets:     resLaunch.Pets,
	}

	return res, nil
}
