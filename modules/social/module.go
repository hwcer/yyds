package social

import (
	"server/game/handle/social/handle"
	"server/game/handle/social/model"

	"github.com/hwcer/cosmo"
)

var Graph = model.Graph
var (
	_ = handle.Register
)

// Start 直接启用嵌入模式，不需要额外配置数据，不需要启用Module
func Start(mo *cosmo.DB, getter model.Handle) error {
	model.SetPlayers(getter)
	model.SetDatabase(mo)
	return nil
}

func Accept(uid, fid string, fast bool) error {
	success, err := model.Graph.Accept(uid, []string{fid}, fast)
	if err != nil {
		return err
	}
	if len(success) == 0 {
		return nil
	}
	bw := model.DB().BulkWrite(&model.Friend{})
	var myFriend []*model.Friend
	for _, tar := range success {
		f1 := model.NewFriend(uid, tar)
		f1.BulkWrite(bw)
		f2 := model.NewFriend(tar, uid)
		f2.BulkWrite(bw)
		myFriend = append(myFriend, f1)
	}

	if err = bw.Submit(); err != nil {
		return err
	}
	return nil
}
