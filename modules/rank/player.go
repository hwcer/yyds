package rank

type Player struct {
	Uid   string `bson:"uid" json:"uid" `                             //UID
	Rank  int64  `bson:"Rank" json:"Rank" `                           //排名,0开始，-1表示未上榜
	Score int64  `bson:"score,omitempty" json:"score" index:"name:" ` //积分
}
