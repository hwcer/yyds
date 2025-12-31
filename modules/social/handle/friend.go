package handle

import (
	"fmt"

	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/slice"

	"github.com/hwcer/cosgo/values"

	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/yyds/context"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/modules/social/graph"
	"github.com/hwcer/yyds/modules/social/model"
	"github.com/hwcer/yyds/players"
	"github.com/hwcer/yyds/players/player"
)

type Friend struct {
}

func (this *Friend) Caller(node *registry.Node, handle *context.Context) interface{} {
	f := node.Method().(func(*Friend, *context.Context) interface{})
	return f(this, handle)
}

const recommendFriendProcessName = "friend_recommend_cache"

func (this *Friend) RecommendReset(c *context.Context) any {
	c.Player.Process.Set(recommendFriendProcessName, []string{})
	return nil
}

/**
 * @name recommendFriend
 * @param int num 拉取的玩家ID数量，1<=num<=100
 * @param string key 额外字段
 * 好友推荐列表
 */
func (this *Friend) Recommend(c *context.Context) any {
	num := c.GetInt("num")
	if num <= 0 {
		return errors.ErrArgEmpty
	}
	if num > 20 {
		num = 20
	}
	var arr []string
	if i := c.Player.Process.Get(recommendFriendProcessName); i != nil {
		arr = i.([]string)
	}
	if len(arr) > 50 {
		arr = slice.Last(arr, 10)
	}
	defer func() {
		if c.Player.Updater.Error == nil {
			c.Player.Process.Set(recommendFriendProcessName, arr)
		}
	}()
	his := map[string]struct{}{}
	his[c.Uid()] = struct{}{} //首先排除自己
	for _, k := range arr {
		his[k] = struct{}{}
	}

	var filter = func(s string) bool {
		_, ok := his[s]
		return !ok
	}

	ids := model.Graph.Recommend(c.Uid(), num, filter, this.recommendAppend)
	if len(ids) == 0 {
		return nil
	}
	rows, err := model.GetPlayers(ids)
	if err != nil {
		return err
	}
	return rows

}

func (this *Friend) recommendAppend(cb graph.RecommendHandle) {
	players.Range(func(s string, _ *player.Player) bool {
		return cb(s)
	})
}

/**
 * @name getter
 * @param string key 额外字段
 * 好友列表
 */

type friendGetterReply struct {
	Uid    string        `json:"uid"`
	Player model.Player  `json:"player"`
	Values values.Values `json:"values"`
}

// Getter 获取好友列表
// update int64 上次更新时间,实现增量更新,如 online 等
// 第一次 update =0
func (this *Friend) Getter(c *context.Context) any {
	uid := c.Uid()
	update := c.GetInt64("update")
	friends := map[string]*friendGetterReply{}
	model.Graph.Range(uid, graph.RelationFriend, func(v graph.Getter) bool {
		p := v.Player()
		if update > 0 && p.Update < update {
			return true
		}
		k := v.Fid()
		f := v.Friend()
		r := &friendGetterReply{
			Uid:    k,
			Values: f.Values.Clone(),
		}
		friends[k] = r
		return true
	})

	var fid []string
	for k, _ := range friends {
		fid = append(fid, k)
	}

	ps, err := model.GetPlayers(fid)
	if err != nil {
		return err
	}
	reply := make([]*friendGetterReply, 0, len(ps))
	for _, p := range ps {
		id := p.GetUid()
		if v := friends[id]; v != nil {
			v.Player = p
			reply = append(reply, v)
		}
	}

	return reply
}

// Fans 我的粉丝 ，等待我确认的
func (this *Friend) Fans(c *context.Context) any {
	uid := c.Uid()
	var ids []string
	model.Graph.Range(uid, graph.RelationFans, func(getter graph.Getter) bool {
		ids = append(ids, getter.Fid())
		return true
	})
	if len(ids) > 10 {
		ids = ids[0:10]
	}
	if r, err := model.GetPlayers(ids); err != nil {
		return err
	} else {
		return r
	}
}

// Follow 我的关注，我申请过的
func (this *Friend) Follow(c *context.Context) any {
	uid := c.Uid()
	var ids []string
	model.Graph.Range(uid, graph.RelationFollow, func(getter graph.Getter) bool {
		ids = append(ids, getter.Fid())
		return true
	})
	if len(ids) > 10 {
		ids = ids[0:10]
	}
	if r, err := model.GetPlayers(ids); err != nil {
		return err
	} else {
		return r
	}
}

/**
 * @name Apply
 * @param string fid 好友ID
 * 添加好友
 */
func (this *Friend) Apply(c *context.Context) any {
	s := c.GetString("fid")
	if s == "" {
		return errors.ErrArgEmpty
	}
	arr := slice.Split(s)
	arr = slice.Unrepeated(arr)
	if l := len(arr); l == 0 {
		return errors.ErrArgEmpty
	} else if l > 10 {
		arr = arr[0:10]
	}
	if len(arr) == 0 {
		return errors.ErrArgEmpty
	}
	uid := c.Uid()
	for _, v := range arr {
		if v == uid {
			return c.Errorf(0, "不能加自己")
		}
	}

	var db = model.DB()

	rows, err := model.GetPlayers(arr)
	if err != nil {
		return err
	}
	bw := db.BulkWrite(&model.Friend{})
	var us []string
	for k, _ := range rows {
		us = append(us, k)
	}

	rst := model.Graph.Follow(uid, us)
	if rst.Error != nil {
		return rst.Error
	}

	var success []*model.Friend
	for _, fid := range rst.Success() {
		f1 := model.NewFriend(uid, fid)
		f1.BulkWrite(bw)
		f2 := model.NewFriend(fid, uid)
		f2.BulkWrite(bw)
		success = append(success, f1)
	}

	if err = bw.Submit(); err != nil {
		return err
	}
	//通知对方 ???? todo
	return success
}

/**
 * @name refuse
 * @param string fid 申请人UID，空：一键拒绝
 * 拒绝申请
 * 拒绝申请时调用此接口直接删除
 * 好友系统不需要验证时，不要调用此接口
 */

func (this *Friend) Refuse(c *context.Context) any {
	fid := c.GetString("fid")
	var arr []string
	if fid != "" {
		arr = slice.Split(fid)
		arr = slice.Unrepeated(arr)
	}
	if len(arr) > 10 {
		arr = arr[0:10]
	}
	if err := model.Graph.Refuse(c.Uid(), arr); err != nil {
		return err
	}
	return true
}

/**
 * @name accept
 * @param string fid 申请人UID,逗号分割多个申请人ID
 * 通过申请
 */

func (this *Friend) Accept(c *context.Context) any {
	fid := c.GetString("fid")
	var arr []string
	if fid != "" {
		arr = slice.Split(fid)
		arr = slice.Unrepeated(arr)
	}
	if len(arr) > 10 {
		arr = arr[0:10]
	}

	rst := model.Graph.Accept(c.Uid(), arr)
	if rst.Error != nil {
		return rst.Error
	}
	success := rst.Success()
	if len(success) == 0 {
		return nil
	}
	uid := c.Uid()
	bw := model.DB().BulkWrite(&model.Friend{})
	var myFriend []*model.Friend
	for _, tar := range success {
		f1 := model.NewFriend(uid, tar)
		f1.BulkWrite(bw)
		f2 := model.NewFriend(tar, uid)
		f2.BulkWrite(bw)
		myFriend = append(myFriend, f1)
	}

	if err := bw.Submit(); err != nil {
		return err
	}
	return myFriend
}

/**
 * @name remove
 * @param string fid 好友UID
 * 删除好友
 */

func (this *Friend) Remove(c *context.Context) any {
	s := c.GetString("fid")
	if s == "" {
		return errors.ErrArgEmpty
	}
	arr := slice.Split(s)
	arr = slice.Unrepeated(arr)

	if l := len(arr); l > 10 {
		arr = arr[0:10]
	}
	uid := c.Uid()
	model.Graph.Remove(uid, arr)

	var keys []string
	fri := &model.Friend{}
	for _, fid := range arr {
		keys = append(keys, fri.ObjectId(uid, fid))
		keys = append(keys, fri.ObjectId(fid, uid))
	}

	if tx := model.DB().Model(fri).Delete(keys); tx.Error != nil {
		return tx.Error
	}
	// 通知对方删除好友
	//u.SendMessage("删除好友",c.Uid())
	return arr
}

/**
 * @name Remark
 * @param string fid 好友ID
 * @param string remark 备注
 * 好友备注
 */

func (this *Friend) Remark(c *context.Context) any {
	fid := c.GetString("fid")
	if fid == "" {
		return errors.ErrArgEmpty
	}
	remark := c.GetString("remark")
	if l := len(remark); l < 2 || l > 30 {
		return c.Errorf(0, "args error")
	}
	if utils.IncludeNotPrintableChar(remark) {
		return c.Errorf(0, "非法字符")
	}

	err := model.Graph.Modify(c.Uid(), func(p *graph.Player) error {
		friend := p.Friend(fid)
		if friend == nil || !friend.Has(graph.RelationFriend) {
			return errors.New("not friend")
		}
		friend.Set(model.FriendValuesKeyRemark, remark)
		return nil
	})

	if err != nil {
		return err
	}

	fri := model.NewFriend(c.Uid(), fid)
	key := fmt.Sprintf("values.%s", model.FriendValuesKeyRemark)
	if tx := model.DB().Model(fri, true).Where(fri.Id).Set(key, remark); tx.Error != nil {
		return tx.Error
	}
	return fri
}
