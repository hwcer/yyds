package model

import (
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"strings"
)

const (
	RoleHandleDailyVal  = "val"
	RoleHandleDailyName = "daily"
)

var roleDailyKeyDict = map[RoleDailyKey]struct{}{}

func init() {
	Handle.Register(RoleHandleDailyName, &RoleDaily{})
}

type RoleDailyKey string

func (key RoleDailyKey) Has() bool {
	_, ok := roleDailyKeyDict[key]
	return ok
}
func (key RoleDailyKey) Mark() {
	if _, ok := roleDailyKeyDict[key]; ok {
		logger.Fatal("RoleDailyKey[%v] exists", key)
	} else {
		roleDailyKeyDict[key] = struct{}{}
	}
}

// RoleDaily 特殊的日常
type RoleDaily struct {
	verify bool
	Expire int64         `json:"ttl" bson:"ttl"` //过期时间
	Values values.Values `json:"val" bson:"val"` //记录  id->val
}

func (rd *RoleDaily) Verify(u *updater.Updater) error {
	u.On(updater.OnPreRelease, rd.Release)
	now := u.Time.Unix()
	if rd.Expire == 0 || rd.Expire < now {
		ts, err := times.Expire(times.ExpireTypeDaily, 1)
		if err != nil {
			return err
		}
		rd.Expire = ts.Unix()
		rd.Values = values.Values{}
	}
	rd.verify = true
	return nil
}

// Release 重置检查状态，自动执行无需调用
func (rd *RoleDaily) Release(u *updater.Updater) bool {
	rd.verify = false
	return false
}

func (rd *RoleDaily) Get(k RoleDailyKey) (r any, ok bool) {
	if !rd.verify {
		logger.Error("RoleDaily必须先使用RoleDaily.Verify对数据进行检查")
	}
	r, ok = rd.Values[string(k)]
	return
}

func (rd *RoleDaily) GetInt(k RoleDailyKey) int {
	return int(rd.GetInt64(k))
}

func (rd *RoleDaily) GetInt32(k RoleDailyKey) int32 {
	return int32(rd.GetInt64(k))
}

func (rd *RoleDaily) GetInt64(k RoleDailyKey) int64 {
	r, ok := rd.Get(k)
	if !ok {
		return 0
	}
	return values.ParseInt64(r)
}

func (rd *RoleDaily) GetString(k RoleDailyKey) string {
	r, ok := rd.Get(k)
	if !ok {
		return ""
	}
	return values.ParseString(r)
}

func (rd *RoleDaily) Unmarshal(k RoleDailyKey, v any) error {
	if !rd.verify {
		logger.Error("RoleDaily必须先使用RoleDaily.Verify对数据进行检查")
	}
	return rd.Values.Unmarshal(string(k), v)
}

// role Handler 方法
func (this *RoleDaily) getter(role *Role, k string) (any, bool) {
	rd := &role.Daily
	if rd.Values == nil {
		rd.Values = values.Values{}
	}
	if !strings.HasPrefix(k, "val.") {
		logger.Error("RoleDaily.Get k is illegal:%v", k)
		return nil, true
	}
	rk := strings.TrimPrefix("val.", k)
	r, ok := rd.Values[rk]
	return r, ok
}
func (this *RoleDaily) setter(role *Role, k string, v any) (any, bool) {
	up := dataset.Update{}
	rd := &role.Daily
	if !rd.verify {
		logger.Error("RoleDaily必须先使用RoleDaily.Verify对数据进行检查")
		return up, true
	}
	if rd.Values == nil {
		rd.Values = values.Values{}
	}

	if !strings.HasPrefix(k, "val.") {
		logger.Error("RoleDaily.setter k Error:%v", k)
		return up, true
	}
	sk := strings.TrimPrefix(k, "val.")
	if !RoleDailyKey(sk).Has() {
		logger.Error("RoleDaily.setter k unknown:%v", k)
		return up, true
	}
	size := len(this.Values)
	rv, err := rd.Values.Marshal(sk, v)
	if err != nil {
		logger.Error("RoleDaily.setter Marshal Error,key:%v,val:%v", k, v)
		return up, true
	}
	if size == 0 {
		up[RoleHandleDailyName] = rd
	} else {
		rk := strings.Join([]string{RoleHandleDailyName, k}, ".")
		up[rk] = rv
	}
	return up, true
}
