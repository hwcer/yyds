package handle

import (
	"server/game/cache"

	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/context"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players/player"
)

const (
	FriendMaxNum = 100
)

func init() {
	//cosrpc.Selector.Set(options.ServiceTypeSocial, options.NewSelector(options.ServiceTypeSocial))
	Register(&Friend{})
}

var Service = context.NewService(options.ServiceTypeSocial)

func Register(i interface{}, prefix ...string) {
	var arr []string
	if options.Gate.Prefix != "" {
		arr = append(arr, options.Gate.Prefix)
	}
	if len(prefix) > 0 {
		arr = append(arr, prefix...)
	} else {
		arr = append(arr, "%v")
	}
	if err := Service.Register(i, arr...); err != nil {
		logger.Fatal("%v", err)
	}
}

func AddLikes(c *context.Context, uid string) {
	_ = c.GetPlayer(uid, false, func(player *player.Player) error {
		doc := cache.GetRole(player.Updater)
		doc.Add("likes", 1)
		_, _ = player.Submit()
		return nil
	})

}

/*
func GetPetsCollectList(gp *graph.Player, today int64) map[string]int64 {
	reply := map[string]int64{}
	var list map[string]int64
	s := gp.Get(socialModel.PlayerValuesKeyCollectGold)
	if s != nil {
		list, _ = s.(map[string]int64)
	} else {
		list = make(map[string]int64)
	}
	for k, v := range list {
		if v > today {
			reply[k] = v
		}
	}
	return reply
}

func GetPetsCollect(uid string, fid string, PetID string) (map[string]int64, error) {
	now := times.Now().Unix()
	reply := map[string]int64{}
	today := times.Daily(0).Now().Unix()
	err := socialModel.Graph.Modify(uid, func(p *graph.Player) error {
		gp := p.Friend(fid)
		if gp == nil {
			return errors.New("对方不是你的好友")
		}
		var list map[string]int64
		s := gp.Get(socialModel.PlayerValuesKeyCollectGold)
		if s != nil {
			list, _ = s.(map[string]int64)
		} else {
			list = make(map[string]int64)
		}
		var update bool
		for k, v := range list {
			if v > today {
				reply[k] = v
			} else {
				update = true
			}
		}
		if PetID != "" {
			if r, ok := reply[PetID]; ok && now-r < 3600 {
				return errors.New("请不要连续薅同一只宠物")
			}
			if len(reply) >= int(config.Data.Base.FriendCollectGold) {
				return errors.New("剩余次数不足")
			}
			update = true
			reply[PetID] = now
		}
		if update {
			gp.Set(socialModel.PlayerValuesKeyCollectGold, reply)
		}
		return nil
	})
	return reply, err
}
*/
