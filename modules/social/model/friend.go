package model

import (
	"fmt"
	"time"

	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/cosmo/update"
	"github.com/hwcer/yyds/modules/social/graph"
)

func init() {
	Register(&Friend{})
}

const (
	FriendValuesKeyRemark = "remark" //好友备注
)

type Friend struct {
	Id       string         `json:"-" bson:"_id"`                       // uid-tar
	Uid      string         `bson:"uid" json:"uid" index:""`            //我的 uid
	Fid      string         `bson:"fid,omitempty" json:"fid" index:""`  //好友 uid
	Player   any            `bson:"player,omitempty" json:"player" `    //好友信息缓存
	Create   int64          `bson:"create,omitempty" json:"create" `    //创建时间
	Update   int64          `bson:"update,omitempty" json:"update" `    //更新时间
	Values   values.Values  `json:"values,omitempty" bson:"values"`     //互动信息
	Relation graph.Relation `json:"relation,omitempty" bson:"relation"` //好友关系
}

func NewFriend(uid, fid string) *Friend {
	r := &Friend{Uid: uid}
	r.Fid = fid
	r.Id = r.ObjectId(uid, fid)
	r.Create = time.Now().Unix()
	return r
}

func (f *Friend) BulkWrite(bw *cosmo.BulkWrite) {
	up := update.Update{}
	now := time.Now().Unix()
	up.Set("update", now)
	up.SetOnInert("uid", f.Uid)
	up.SetOnInert("fid", f.Fid)
	up.SetOnInert("create", now)
	bw.Update(up, f.Id)
}

func (f *Friend) ObjectId(uid, tar string) string {
	return fmt.Sprintf("%s-%s", uid, tar)
}
