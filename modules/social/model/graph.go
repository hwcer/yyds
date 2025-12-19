package model

import (
	"server/game/handle/social/graph"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/logger"
)

const (
	FriendValuesKeyLike   = "like"   //点赞
	FriendValuesKeyRemark = "remark" //好友备注
	//PlayerValuesKeyOnline      = "online" //是否在线
	//PlayerValuesKeyCollectGold = "CollectGold"
	//PlayerValuesKeyEggFast     = "EggFast"
)

var Graph, install = graph.New(factory{})

func init() {
	cosgo.On(cosgo.EventTypLoaded, graphInstall)
	//players.On(players.EventConnect, playerConnect)
	//players.On(players.EventDisconnect, playerDisconnect)
}

//func playerConnect(u *updater.Updater, v ...int32) {
//	_ = Graph.Modify(u.Uid(), func(p *graph.Player) error {
//		p.Set(PlayerValuesKeyOnline, int32(1))
//		return nil
//	})
//}

//func playerDisconnect(u *updater.Updater, v ...int32) {
//	_ = Graph.Modify(u.Uid(), func(p *graph.Player) error {
//		p.Set(PlayerValuesKeyOnline, int32(0))
//		return nil
//	})
//}

//func NewNotify(id string) (graph.Player, error) {
//	return &Notify{uid: id}, nil
//}

type factory struct {
}

func (factory) Create(uid string) (values.Values, error) {
	return values.Values{}, nil
}

func (factory) SendMessage(uid string, path string, data any) {
	logger.Alert("好友消息推送,uid:%s,Path:%s,Data:%v", uid, path, data)
}

//type FWBossShare struct {
//	Id     string `json:"id" `
//	HP     int64  `json:"hp"`
//	Expire int64  `json:"expire"`
//}
//
//type Notify struct {
//	uid    string
//	Helps  string       //助阵列表
//	FWBoos *FWBossShare //好友BOSS
//}

func graphInstall() (err error) {
	var n int
	logger.Trace("开始加载好友关系")
	defer func() {
		logger.Trace("累计加载好友关系 %d 条", n)
	}()
	tx := db.Model(&Friend{})
	//today := times.Daily(0).Now().Unix()
	tx.Range(func(cursor cosmo.Cursor) bool {
		fri := &Friend{}
		if err = cursor.Decode(fri); err != nil {
			return false
		}
		if _, err = install.SetFriend(fri.Uid, fri.Fid, graph.RelationFriend, fri.Values); err != nil {
			return false
		}

		n += 1
		return true
	})

	return
}
