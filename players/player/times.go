package player

import (
	"fmt"
	"time"

	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players/expire"
)

type Times struct {
	p *Player
}

// Start 开始时间
//
//	args [开始，结束], 这里结束可以省略，仅仅是为了保持参数一致性
func (this *Times) Start(t int64, args []int64) (r int64, err error) {
	if len(args) < 1 {
		return 0, errors.ErrArgEmpty
	}
	v := args[0]
	if v == 0 {
		v = 1
	}
	et := times.ExpireType(t)
	if et.Has() {
		var ts *times.Times
		if ts, err = times.Start(et, int(v)); err == nil && ts != nil {
			r = ts.Now().Unix()
		}
		return
	}

	switch et {
	case expire.TimePlayerCreate:
		role := this.p.Document(RoleIType)
		create := role.Get(RoleFields.Create)
		dt := times.Unix(values.ParseInt64(create)).Daily(int(v - 1))
		return dt.Now().Unix(), nil
	case expire.TimeServerCreate:
		dt := times.Unix(options.GetServerTime())
		dt = dt.Daily(int(v - 1))
		return dt.Now().Unix(), nil
	default:
		if h := expire.Get(et); h != nil {
			return h.Start(this.p.Updater, t, args)
		}
		err = values.Errorf(0, "time type unknown")
		return
	}

}

// Expire 过期时间点，语义为左闭右开区间 [Start, Expire) 的右端点
//
// 该时间点【本身不属于】本届：判定有效一律用 now < expire，判定过期用 now >= expire。
// 口径与 cosgo/times.Times.Expire 一致，勿在调用方用 <= / >= 判定，否则会晚一个单位过期。
//
//	args [开始，结束]
func (this *Times) Expire(t int64, args []int64) (r int64, err error) {
	if t == 0 {
		return 0, nil
	}
	//args 缺结束时间时按 0 处理,与配置行 end=0 同语义:daily/weekly/monthly 由
	//cosgo 归一为"默认1天/周/月",second 为立即过期,8/9 为无结束时间
	v := int64(0)
	if len(args) >= 2 {
		v = args[1]
	}

	et := times.ExpireType(t)
	if et.Has() {
		var ts *times.Times
		if ts, err = times.Expire(et, int(v)); err == nil && ts != nil {
			r = ts.Now().Unix()
		}
		return
	}
	switch et {
	case expire.TimePlayerCreate:
		if v > 0 {
			role := this.p.Document(RoleIType)
			create := role.Get(RoleFields.Create)
			dt := times.Unix(values.ParseInt64(create)).Daily(int(v))
			r = dt.Now().Unix()
		}
		return
	case expire.TimeServerCreate:
		if v > 0 {
			dt := times.Unix(options.GetServerTime())
			dt = dt.Daily(int(v))
			r = dt.Now().Unix()
		}
		return
	default:
		if h := expire.Get(et); h != nil {
			return h.Expire(this.p.Updater, t, args)
		}
		err = values.Errorf(0, "time type unknown")
		return
	}
}

// Verify 验证是否在有效期(开始以及过期时间)内，返回开始和结束时间
//
// 区间为左闭右开 [start, expire)：start 那一刻已开始，expire 那一刻已结束（见 Expire 注释）。
func (this *Times) Verify(args []int64) (t [2]int64, err error) {
	arr := []int64{0, 0, 0}
	copy(arr, args)
	now := time.Now().Unix()
	if t[0], err = this.Start(arr[0], arr[1:]); err != nil {
		return
	} else if t[0] > now {
		err = errors.ErrActiveDisable
		return
	}
	if t[1], err = this.Expire(arr[0], arr[1:]); err != nil {
		return
	}
	if t[1] > 0 && t[1] <= now {
		err = errors.ErrActiveExpired
		return
	}
	return
}

func (this *Times) isCycle(t int64) bool {
	if et := times.ExpireType(t); et == times.ExpireTypeDaily || et == times.ExpireTypeWeekly || et == times.ExpireTypeMonthly {
		return true
	}

	return false
}

// Cycle 活动周期
//
//	expire 活动开始结束时间 [0,0,0]
//	refresh 活动刷新规则  [0,0],参考  times.Cycle
func (this *Times) Cycle(expire []int64, refresh []int64) (cycle *times.Cycle, err error) {
	var ts [2]int64
	//开启时间检查
	if ts, err = this.Verify(expire); err != nil {
		return
	}

	//周期
	refreshRule := []int64{0, 0}
	copy(refreshRule, refresh)

	if ts[0] == 0 && this.isCycle(refreshRule[0]) && refreshRule[1] > 1 {
		return nil, fmt.Errorf("start time empty")
	}

	var st *times.Times
	if ts[0] == 0 {
		st = times.Daily(0)
	} else {
		st = times.Unix(ts[0])
	}
	//有效期,如果未设置有效期，而设置了活动时间，则使用整个活动周期
	if refreshRule[0] == 0 && ts[0] > 0 && ts[1] > ts[0] {
		refreshRule[0] = int64(times.ExpireTypeSecond)
		refreshRule[1] = ts[1] - ts[0]
	}
	cycle = st.Cycle(times.ExpireType(refreshRule[0]), int(refreshRule[1]))
	return
}

// StartWithArray / ExpireWithArray 变参包装:args 即配置行 [t,开始,结束,...],
// t 之外整体透传(与 Verify 同约定),多余的参数留给自定义 handler
func (this *Times) StartWithArray(args ...int64) (r int64, err error) {
	if len(args) == 0 {
		return 0, nil
	}
	return this.Start(args[0], args[1:])
}

func (this *Times) ExpireWithArray(args ...int64) (r int64, err error) {
	if len(args) == 0 {
		return 0, nil
	}
	return this.Expire(args[0], args[1:])
}
