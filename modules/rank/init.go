package rank

import (
	"github.com/hwcer/cosgo/redis"
)

type SortType int8

const (
	SortTypeAsc  SortType = -1
	SortTypeDesc SortType = 1
)

// 排行榜系统
const (
	Heartbeat         = 5   //心跳间隔(s)
	OverflowThreshold = 500 //排行榜人数超过预设值N个时触发清理
)

var Options = struct {
	ShareId   string
	ServerId  int32
	StartTime string //开服时间
}{
	StartTime: "2024-01-01 00:00:00+0800",
}
var Redis *redis.Client

type Handle interface {
	Truce() int64                             //赛季前X秒进入休战期，休战期开始结算，并且无法再更新数据
	Cycle(skip int64) (cycle int64)           //返回本期标记,skip :0 当前，-1：上一届，1：下一届。。。
	Expire(cycle int64) (start, expire int64) //当前界排行榜开始时间，有效期时间(s)
	Submit(b *Bucket, cycle int64) error      //结算
}

type HandleHeartbeat interface {
	Heartbeat(w *Bucket, cycle int64)
}
