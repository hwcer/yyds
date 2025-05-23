package player

import (
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
)

// 更多参考 times.ExpireType

const (
	ExpireTimePlayerCreate times.ExpireType = 8
	ExpireTimeServerCreate times.ExpireType = 9
)

type Times struct {
	p *Player
}

// Start 开始时间
func (this *Times) Start(t int64, v int64) (r int64, err error) {
	if v == 0 {
		v = 1
	}
	et := times.ExpireType(t)
	switch et {
	case times.ExpireTypeNone:
		return
	case times.ExpireTypeDaily:
		return times.Daily(0).Now().Unix(), nil
	case times.ExpireTypeWeekly:
		return times.Weekly(0).Now().Unix(), nil
	case times.ExpireTypeMonthly:
		return times.Monthly(0).Now().Unix(), nil
	case times.ExpireTypeSecond:
		return v, nil
	case times.ExpireTypeCustomize:
		var ttl *times.Times
		if ttl, err = times.ParseExpireTypeCustomize(int(v)); err == nil {
			r = ttl.Now().Unix()
		}
		return
	case times.ExpireTimeTimeStamp:
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
	return this.Expire(expire[0], expire[1])
}

// Expire 过期时间
func (this *Times) Expire(t int64, v int64) (r int64, err error) {
	if t == 0 {
		return 0, nil
	}
	if v == 0 {
		v = 1
	}
	et := times.ExpireType(t)
	if et.Has() {
		var ts *times.Times
		if ts, err = times.Expire(et, int(v)); err == nil {
			r = ts.Now().Unix()
		}
		return
	}

	switch et {
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
	default:
		err = values.Errorf(0, "time type unknown")
		return
	}
}

// Verify 验证是否在有效期(开始以及过期时间)内，返回开始和结束时间
func (this *Times) Verify(args []int64) (t [2]int64, err error) {
	arr := append([]int64{}, args...)
	arr = append(arr, 0, 0, 0)
	now := times.Now().Unix()
	if t[0], err = this.Start(arr[0], arr[1]); err != nil {
		return
	} else if t[0] > now {
		err = errors.ErrActiveDisable
		return
	}
	if t[1], err = this.Expire(arr[0], arr[2]); err != nil {
		return
	}
	if t[1] > 0 && t[1] < now {
		err = errors.ErrActiveExpired
		return
	}
	return
}
