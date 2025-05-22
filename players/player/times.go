package player

import (
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
)

const (
	ExpireTimeNone         int32 = 0
	ExpireTimeDaily        int32 = 1
	ExpireTimeWeekly       int32 = 2
	ExpireTimeMonthly      int32 = 3
	ExpireTimeTimeStamp    int32 = 4
	ExpireTimePlayerCreate int32 = 5
	ExpireTimeServerCreate int32 = 6
	ExpireTimeServerAlways int32 = 9 //终身
)

type Times struct {
	p *Player
}

// Start 开始时间
func (this *Times) Start(t int32, v int64) (r int64, err error) {
	if v < 1 {
		v = 1
	}
	switch t {
	case ExpireTimeNone:
		return
	case ExpireTimeDaily:
		return times.Daily(0).Now().Unix(), nil
	case ExpireTimeWeekly:
		return times.Weekly(0).Now().Unix(), nil
	case ExpireTimeMonthly:
		return times.Monthly(0).Now().Unix(), nil
	case ExpireTimeTimeStamp:
		return v, nil
	case ExpireTimePlayerCreate:
		role := this.p.Document(ITypeRole)
		create := role.Get(Fields.Create)
		dt := times.Unix(values.ParseInt64(create)).Daily(int(v - 1))
		return dt.Now().Unix(), nil
	case ExpireTimeServerCreate:
		dt := times.Unix(options.GetServerTime())
		dt = dt.Daily(int(v - 1))
		return dt.Now().Unix(), nil
	case ExpireTimeServerAlways:
		return 1, nil
	default:
		err = values.Errorf(0, "time type unknown")
		return
	}

}
func (this *Times) ExpireWithArray(expire ...int64) (r int64, err error) {
	if len(expire) == 0 {
		return 0, nil
	}
	if len(expire) < 2 {
		expire = append(expire, 1)
	}
	return this.Expire(int32(expire[0]), expire[1])
}

// Expire 过期时间
func (this *Times) Expire(t int32, v int64) (r int64, err error) {
	switch t {
	case ExpireTimeNone:
		return
	case ExpireTimeDaily, ExpireTimeWeekly, ExpireTimeMonthly:
		var ts *times.Times
		if v == 0 {
			v = 1
		}
		if ts, err = times.Expire(times.ExpireType(t), int(v)); err == nil {
			r = ts.Now().Unix()
		}
		return
	case ExpireTimeTimeStamp:
		return v, nil
	case ExpireTimePlayerCreate:
		if v > 0 {
			role := this.p.Document(ITypeRole)
			create := role.Get(Fields.Create)
			dt := times.Unix(values.ParseInt64(create)).Daily(int(v)).Add(-1)
			r = dt.Now().Unix()
		}
		return
	case ExpireTimeServerCreate:
		if v > 0 {
			dt := times.Unix(options.GetServerTime())
			dt = dt.Daily(int(v)).Add(-1)
			r = dt.Now().Unix()
		}
		return
	case ExpireTimeServerAlways:
		return times.Now().Unix() + times.DaySecond*365*100, nil
	default:
		err = values.Errorf(0, "time type unknown")
		return
	}
}

// Verify 验证是否在有效期(开始以及过期时间)内，返回开始和结束时间
func (this *Times) Verify(args []int64) (t [2]int64, err error) {
	args = append(args, 0, 0, 0)
	now := times.Now().Unix()
	if t[0], err = this.Start(int32(args[0]), args[1]); err != nil {
		return
	} else if t[0] > now {
		err = errors.ErrActiveDisable
		return
	}
	if t[1], err = this.Expire(int32(args[0]), args[2]); err != nil {
		return
	}
	if t[1] > 0 && t[1] < now {
		err = errors.ErrActiveExpired
		return
	}
	return
}
