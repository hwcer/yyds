package verify

import (
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
)

func init() {
	Register(ConditionNone, taskTargetHandleNone)
	Register(ConditionData, taskTargetHandleData)
	Register(ConditionEvents, taskTargetHandleEvents)
	Register(ConditionMethod, taskTargetHandleMethod)
	Register(ConditionWeekly, taskTargetHandleWeekly)
	Register(ConditionHistory, taskTargetHandleHistory)
}
func value(u *updater.Updater, target Value) (r int64) {
	if f, ok := verifyCondition[target.GetCondition()]; ok {
		r = f(u, target)
	} else {
		logger.Alert("Condition unknown,Condition:%v,Key:%v", target.GetCondition(), target.GetKey())
	}
	return
}

func verify(u *updater.Updater, target Target) error {
	var ok bool
	var val int64
	switch target.GetCondition() {
	case ConditionNone:
		ok = true
	default:
		val = value(u, target)
		ok = taskTargetCompare(target, val)
	}
	if !ok {
		if ef, _ := target.(Errorf); ef != nil {
			return ef.Errorf(val)
		} else {
			return ErrGoalNotAchieved
		}
	} else {
		return nil
	}
}

func taskTargetHandleNone(u *updater.Updater, target Value) (r int64) {
	if d, ok := target.(GetVal); ok {
		r = d.GetVal()
	} else {
		u.Error = ErrTargetMethodNotFound
	}
	return
}
func taskTargetHandleEvents(u *updater.Updater, target Value) (r int64) {
	if d, ok := target.(GetVal); ok {
		r = d.GetVal()
	} else {
		logger.Alert("taskTargetHandleEvents ")
	}
	return
}
func taskTargetHandleMethod(u *updater.Updater, target Value) int64 {
	key := target.GetKey()
	if i := GetMethod(key); i != nil {
		return i.Value(u, target)
	} else {
		logger.Alert("Method[%v] not register", key)
	}
	return 0
}

func taskTargetHandleData(u *updater.Updater, target Value) int64 {
	return u.Val(target.GetKey())
}

// daily week
func taskTargetHandleWeekly(u *updater.Updater, target Value) (r int64) {
	k := target.GetKey()
	week := times.Weekly(0)
	r, u.Error = Options.Count(u, k, week, nil)
	return
}

// daily history
func taskTargetHandleHistory(u *updater.Updater, target Value) (r int64) {
	var ts [2]int64
	if i, ok := target.(GetTimes); ok {
		ts = i.GetTimes()
	}
	k := target.GetKey()

	var st, et *times.Times
	if ts[0] > 0 {
		st = times.Unix(ts[0])
	}
	if ts[1] > 0 {
		et = times.Unix(ts[1])
	}
	r, u.Error = Options.Count(u, k, st, et)
	return
}

// taskTargetCompare 比较
func taskTargetCompare(target Target, val int64) bool {
	var compare int32
	if f, ok := target.(GetCompare); ok {
		compare = f.GetCompare()
	}
	goal := int64(target.GetGoal())
	switch compare {
	case CompareGte:
		return val >= goal
	case CompareLte:
		return val <= goal
	default:
		return false
	}
}
