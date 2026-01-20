package model

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/modules/graph"
)

var Graph, install = graph.New(factory{})

func init() {
	cosgo.On(cosgo.EventTypLoaded, graphInstall)
}

type factory struct {
}

func (factory) Create(uid string) (values.Values, error) {
	return values.Values{}, nil
}

func (factory) SendMessage(uid string, path string, data any) {
	logger.Alert("好友消息推送,uid:%s,Path:%s,Data:%v", uid, path, data)
}

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
