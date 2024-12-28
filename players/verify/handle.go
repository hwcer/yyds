package verify

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/times"
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
	switch target.GetCondition() {
	case ConditionNone:
		ok = true
	default:
		ok = taskTargetCompare(target, value(u, target))
	}
	if !ok {
		return ErrGoalNotAchieved
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
	r, u.Error = Options.Count(u, k, week.Unix(), 0)
	return
}

// daily history
func taskTargetHandleHistory(u *updater.Updater, target Value) (r int64) {
	var ts [2]int64
	if i, ok := target.(GetTimes); ok {
		ts = i.GetTimes()
	}
	k := target.GetKey()
	r, u.Error = Options.Count(u, k, ts[0], ts[1])
	return
}

// taskTargetCompare 比较
func taskTargetCompare(target Target, val int64) bool {
	var compare int32
	if f, ok := target.(GetCompare); ok {
		compare = f.GetCompare()
	}
	switch compare {
	case CompareGte:
		return val >= int64(target.GetGoal())
	case CompareLte:
		return val <= int64(target.GetGoal())
	default:
		return false
	}
}
