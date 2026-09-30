package condition

import (
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/errors"
)

func init() {
	Register(TypeNone, taskTargetHandleNone)
	Register(TypeData, taskTargetHandleData)
	Register(TypeEvents, taskTargetHandleEvents)
	Register(TypeMethod, taskTargetHandleMethod)
	Register(TypeWeekly, taskTargetHandleWeekly)
	Register(TypeHistory, taskTargetHandleHistory)
}

// value 获取任务当前进度，若实现了 Judge 接口则对原始值与 ARGS 进行比较后返回
// 如果使用了裁决（Judge） 返回值是裁决结果(0/1)
// 自定义统计函数（TypeMethod）不会裁决，因为是不是需要裁决可以也应该在统计函数中自己决定
func value(u *updater.Updater, target Target) (r int64) {
	t := target.GetCondition()
	if f, ok := handles[t]; ok {
		r = f(u, target)
	} else {
		logger.Alert("Condition unknown,Condition:%v,Key:%v", target.GetCondition(), target.GetKey())
	}
	if judge, ok := target.(Judge); ok && t != TypeMethod {
		if j := judge.GetJudge(); j == JudgeNone {
			//保持原值，不进行裁决
		} else if taskJudgeCompare(j, int32(r), judge.GetArgs()) {
			r = 1
		} else {
			r = 0
		}
	}
	return
}

// verify 验证目标条件是否达成
func verify(u *updater.Updater, target Target) error {
	val := value(u, target)
	ok := taskTargetCompare(target, val)
	if ok {
		return nil
	}
	if ef, _ := target.(Errorf); ef != nil {
		return ef.Errorf(val)
	}
	return errors.ErrGoalNotAchieved
}

func taskTargetHandleNone(u *updater.Updater, target Target) (r int64) {
	return int64(target.GetGoal())
}
func taskTargetHandleEvents(_ *updater.Updater, target Target) (r int64) {
	if d, ok := target.(GetVal); ok {
		r = d.GetVal()
	} else {
		logger.Alert("taskTargetHandleEvents target not implement GetVal,Key:%v", target.GetKey())
	}
	return
}
func taskTargetHandleMethod(u *updater.Updater, target Target) int64 {
	key := target.GetKey()
	if i := GetMethod(key); i != nil {
		return i.Value(u, target)
	}
	logger.Alert("Method[%v] not register", key)
	return 0
}

func taskTargetHandleData(u *updater.Updater, target Target) int64 {
	return u.Val(target.GetKey())
}

// daily week
func taskTargetHandleWeekly(u *updater.Updater, target Target) (r int64) {
	k := target.GetKey()
	week := times.Weekly(0)
	r, err := Options.Count(u, k, week, nil)
	if err != nil {
		_ = u.Errorf(err)
	}

	return
}

// daily history
func taskTargetHandleHistory(u *updater.Updater, target Target) (r int64) {
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
	r, err := Options.Count(u, k, st, et)
	if err != nil {
		_ = u.Errorf(err)
	}
	return
}

// taskJudgeCompare 根据 Judge 类型将 val 与 args 比较，返回成功或者失败
func taskJudgeCompare(judge int32, val int32, args []int32) bool {
	var ok bool
	switch judge {
	case JudgeNone:
		ok = true
	case JudgeEqual:
		ok = len(args) > 0 && val == args[0]
	case JudgeGte:
		ok = len(args) > 0 && val >= (args[0])
	case JudgeLte:
		ok = len(args) > 0 && val <= (args[0])
	case JudgeContains:
		for _, arg := range args {
			if val == (arg) {
				ok = true
				break
			}
		}
	case JudgeRange:
		ok = len(args) > 1 && val >= (args[0]) && val <= (args[1])
	default:
		//与上面 value() 对未知 Condition 的处理对称：fail-closed 之后表现是"条件永远不达成"，
		//不打日志就只能靠猜。绝大多数是配表 Judge 列填错。
		logger.Alert("Judge unknown,Judge:%v,Val:%v,Args:%v", judge, val, args)
		ok = false // 未知的 Judge 类型直接返回0
	}

	return ok
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
