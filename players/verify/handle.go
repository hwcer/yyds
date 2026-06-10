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
// value 获取任务当前进度，若实现了 Judge 接口则对原始值与 ARGS 进行比较后返回
func value(u *updater.Updater, target Value) (r int64) {
	if f, ok := verifyCondition[target.GetCondition()]; ok {
		r = f(u, target)
	} else {
		logger.Alert("Condition unknown,Condition:%v,Key:%v", target.GetCondition(), target.GetKey())
	}
	if j, ok := target.(Judge); ok && j.GetJudge() > JudgeNone {
		r = taskJudgeCompare(j.GetJudge(), r, target.GetArgs())
	}
	return
}

// verify 验证目标条件是否达成
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
	if ok {
		return nil
	}
	if ef, _ := target.(Errorf); ef != nil {
		return ef.Errorf(val)
	}
	return ErrGoalNotAchieved
}

func taskTargetHandleNone(u *updater.Updater, target Value) (r int64) {
	if d, ok := target.(GetVal); ok {
		r = d.GetVal()
	} else {
		u.Error = ErrTargetMethodNotFound
	}
	return
}
func taskTargetHandleEvents(_ *updater.Updater, target Value) (r int64) {
	if d, ok := target.(GetVal); ok {
		r = d.GetVal()
	} else {
		logger.Alert("taskTargetHandleEvents target not implement GetVal,Key:%v", target.GetKey())
	}
	return
}
func taskTargetHandleMethod(u *updater.Updater, target Value) int64 {
	key := target.GetKey()
	if i := GetMethod(key); i != nil {
		return i.Value(u, target)
	}
	logger.Alert("Method[%v] not register", key)
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

// taskJudgeCompare 根据 Judge 类型将 val 与 args 比较，匹配返回1，否则返回0
func taskJudgeCompare(judge int32, val int64, args []int32) int64 {
	var ok bool
	switch judge {
	case JudgeEqual:
		ok = len(args) > 0 && val == int64(args[0])
	case JudgeGte:
		ok = len(args) > 0 && val >= int64(args[0])
	case JudgeLte:
		ok = len(args) > 0 && val <= int64(args[0])
	case JudgeContains:
		for _, arg := range args {
			if val == int64(arg) {
				ok = true
				break
			}
		}
	case JudgeRange:
		ok = len(args) > 1 && val >= int64(args[0]) && val <= int64(args[1])
	default:
		return val
	}
	if ok {
		return 1
	}
	return 0
}

// taskTargetCompare 目标比较
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
