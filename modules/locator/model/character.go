package model

func init() {
	db.Register(&Character{})
}

type Character struct {
	Uid  string `json:"uid" bson:"_id"`
	Sid  int32  `json:"sid" bson:"sid"`            //服务器ID
	Guid string `bson:"guid" json:"guid" index:""` //账号ID

	Lv   int32  `json:"lv" bson:"lv"`               //等级
	Name string `json:"name" bson:"name"`           //名称
	Icon string `json:"icon,omitempty" bson:"icon"` //头像
	Prof int32  `json:"prof,omitempty" bson:"prof"` //职业

	Online int64  `json:"online,omitempty" bson:"online" index:""`                               //最后登陆时间
	Create int64  `json:"create,omitempty" bson:"create" `                                       //创建时间
	Invite string `json:"invite,omitempty" bson:"invite" index:"PARTIAL:invite.length > int(0)"` //邀请我的人
}

func (this *Character) Fields() []string {
	return []string{"lv", "name", "icon", "prof"}
}
