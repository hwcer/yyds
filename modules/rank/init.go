package rank

import (
	"errors"
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

// SwapCond ZSwap 的交换条件,在 Lua 内部重查后判定,保证"读-判-写"整体原子
type SwapCond int8

const (
	SwapAlways        SwapCond = 0 //无条件对调
	SwapIfTargetAhead SwapCond = 1 //仅当 b 比 a 更靠前时才对调(按该榜 SortType 判定)
)

// ZSwap 的错误,业务侧需按 errors.Is 区分处理
var (
	//ErrSwapMemberMissing 至少一方不在榜上(退榜/换届/从未入榜)
	ErrSwapMemberMissing = errors.New("rank: member not on the list")
	//ErrSwapCondition 交换条件不满足,如要求目标更靠前但其名次已被他人换走
	ErrSwapCondition = errors.New("rank: swap condition not satisfied")
	//ErrTruce 休战期内不可写
	//
	//注意与 ZAdd 的差异:ZAdd 在休战期静默返回 nil(丢弃写入),那是分数榜可接受的降级;
	//但交换类操作静默失败会导致业务侧以为换成功了(已扣代价却没换),故必须显式报错。
	ErrTruce = errors.New("rank: truce period, list is read-only")
)

// Option 注册排行榜时的可选项
type Option func(*Bucket)

// WithoutTiebreak 关闭同分 tiebreak,令 REDIS score 保持纯整数
//
// 默认 formatScore 会把"本届已过时间"编码进 score 的小数位,用于同分时"先达成者排前"。
// 当分数本身即唯一序号时(如名次席位榜:score 就是名次,天然不可能同分),这层编码纯属多余:
// 读侧必须处处记得 parseScore 还原,一旦有路径直接拿 ZSCORE 的浮点去比较就会出错;
// 若还存在绕开 formatScore 的写入路径(如 Lua 脚本直接 ZADD),同一个榜里会并存两种分数格式。
func WithoutTiebreak() Option {
	return func(b *Bucket) { b.zTiebreak = false }
}

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
