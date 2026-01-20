package rank

import (
	"fmt"
	"strconv"
	"time"
)

func NewStatement(zCycle, zTime, zExpire int64) *Statement {
	if zTime == 0 {
		zTime = eraYear
	}
	return &Statement{zCycle: zCycle, zTime: zTime, zExpire: zExpire}
}

type Statement struct {
	zTime   int64 //本期开始时间
	zExpire int64 //过期时间
	zCycle  int64 //当前使用的期数
	zKeeper int64 //守门员
	submit  bool  //是否已经结算
	//zMember *sync.Map //用户信息 //uid =>Rank  //优化.优化
}

func (this *Statement) formatScore(w *Bucket, score int64) (r float64) {
	var t int64
	now := time.Now().Unix()
	if w.zType == SortTypeDesc {
		if this.zExpire > 0 && this.zTime > 0 && this.zTime+this.zExpire > now {
			t = this.zTime + this.zExpire - now
		} else {
			t = doomsday - now
		}
	} else {
		t = now - this.zTime
	}
	if t <= 0 {
		return float64(score)
	}
	v := fmt.Sprintf("%v.%v", score, t)
	var err error
	if r, err = strconv.ParseFloat(v, 10); err != nil {
		// 解析失败时返回原始分数，避免排行榜数据异常
		return float64(score)
	}
	return
}
