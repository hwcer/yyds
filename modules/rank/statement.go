package rank

import (
	"math"
	"sync/atomic"
	"time"
)

func NewStatement(zCycle, zTime, zExpire int64) *Statement {
	if zTime == 0 {
		zTime = eraYear
	}
	return &Statement{zCycle: zCycle, zTime: zTime, zExpire: zExpire}
}

// Statement 一届排行榜的状态
//
// zTime/zExpire/zCycle 构造后只读; zKeeper/submit 由心跳协程写、业务协程读,必须用原子操作
type Statement struct {
	zTime   int64        //本期开始时间(unix秒)
	zExpire int64        //本期结束时间(unix秒)
	zCycle  int64        //当前使用的期数
	zKeeper atomic.Int64 //守门员分数,0表示未设置
	submit  atomic.Bool  //是否已经结算
	//zMember *sync.Map //用户信息 //uid =>Rank  //优化.优化
}

// formatScore 把分数和达成时间打包成 REDIS ZSET 的 score
//
// 整数部分为原始分数,小数部分编码本届已过时间,用于同分时"先达成者排前"。
// float64 精度是相对的,分数越大留给时间的尾数位越少,tiebreak 粒度自动变粗,
// 但排序正确性与分数量级无关。解码用 parseScore。
//
// 30天周期下 score <= 42.9亿 仍可区分到1秒;分数上限 2^53-1(9007199254740991),
// 到达 2^53 时相邻分数不再可分。超过 2^52 时 tiebreak 已完全退化为并列。
func (this *Statement) formatScore(w *Bucket, score int64) float64 {
	base := float64(score)
	d := this.zExpire - this.zTime //本届时长
	if d <= 1 {
		return base //时长无效,不做时间tiebreak
	}
	elapsed := time.Now().Unix() - this.zTime
	if elapsed < 0 {
		elapsed = 0
	} else if elapsed > d-1 {
		elapsed = d - 1
	}
	off := elapsed
	if w.zType == SortTypeDesc {
		off = d - 1 - elapsed //降序:达成越早剩余越多,排名越靠前
	}
	//钳位到 score 的下一个整数之前:保证 floor 恒等于 score,且同分内单调不反转。
	//注意不能改成"进位就丢弃小数",那样同分玩家一部分丢一部分留,顺序会反。
	return math.Min(base+float64(off)/float64(d), math.Nextafter(base+1, base))
}

// parseScore 从 REDIS ZSET 的 score 还原原始分数
//
// float64→int64 越界时行为未定义(amd64上会翻转成MinInt64),必须先饱和截断
func parseScore(score float64) int64 {
	f := math.Floor(score)
	if f >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	if f <= float64(math.MinInt64) {
		return math.MinInt64
	}
	return int64(f)
}
