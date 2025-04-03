package model

func init() {
	db.Register(&Character{})
}

type Character struct {
	Uid    string `json:"_id" bson:"_id"`
	Lv     int32  `json:"lv" bson:"lv"`                                                //等级
	Sid    int32  `json:"sid" bson:"sid"`                                              //服务器ID
	Guid   string `bson:"guid" json:"guid" index:""`                                   //账号ID
	Name   string `json:"name" bson:"name"`                                            //名称
	Icon   string `json:"icon" bson:"icon"`                                            //头像
	Prof   int32  `json:"prof" bson:"prof"`                                            //职业
	Create int64  `json:"create" bson:"create" `                                       //创建时间
	Update int64  `json:"update" bson:"update"`                                        //最后登陆时间
	Invite string `json:"invite" bson:"invite" index:"PARTIAL:invite.length > int(0)"` //邀请我的人
}
