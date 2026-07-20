package rank

import (
	"math"
	"time"

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
	//DefaultRetention 无法计算周期长度时,结算后数据的兜底保留时长
	DefaultRetention = 7 * 24 * time.Hour
)

// ScoreUnlimited 用于 Register 的 zScore 参数,表示不限制入围分数
//
// 注意不能用0表示不限制:0是合法的入围门槛(如降序榜只收非负分),
// 传0的含义是"分数必须>=0(降序)或<=0(升序)"
const ScoreUnlimited int64 = math.MinInt64

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
	Expire(cycle int64) (start, expire int64) //当前届排行榜的开始时间和结束时间,均为unix秒
	//Submit 结算,返回排行榜数据的删除时间(unix秒)
	//
	//返回值小于当前时间时(如返回0)使用 DefaultRetention 兜底
	Submit(b *Bucket, cycle int64) (expire int64, err error)
}

type HandleHeartbeat interface {
	Heartbeat(w *Bucket, cycle int64)
}
