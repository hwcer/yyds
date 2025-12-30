package social

import (
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/modules/social/handle"
	"github.com/hwcer/yyds/modules/social/model"
)

var Graph = model.Graph

// Start 直接启用嵌入模式，不需要额外配置数据，不需要启用Module
func Start(service *registry.Service, mo *cosmo.DB, getter model.Handle) error {
	model.SetPlayers(getter)
	model.SetDatabase(mo)
	return service.Register(&handle.Friend{})
}

// Accept 快速通过好友，不需要确认绑定好友关系
// 如果设置了 好友上限，可能会失败
func Accept(uid, fid string, fast bool) error {
	success, err := model.Graph.Accept(uid, []string{fid}, fast)
	if err != nil {
		return err
	}
	if len(success) == 0 {
		return nil
	}
	db := model.DB()
	if db == nil {
		return errors.Error("social database empty")
	}
	bw := db.BulkWrite(&model.Friend{})
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
