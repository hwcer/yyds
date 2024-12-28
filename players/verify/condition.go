package verify

import "github.com/hwcer/updater"

const (
	ConditionNone    int32 = 0   //无条件直接返回成功
	ConditionData    int32 = 1   //基础数据,日常，成就记录
	ConditionEvents  int32 = 2   //即时任务，监听数据,仅限于任务
	ConditionMethod  int32 = 9   //需要方法实现
	ConditionWeekly  int32 = 101 //周数据,基于daily
	ConditionHistory int32 = 102 //历史数据
)

var verifyCondition = make(map[int32]verifyConditionHandle)

// verifyConditionHandle times  开始时间，结束时间仅仅用在 TaskConditionHistory 类型的活动中
type verifyConditionHandle func(u *updater.Updater, handle Value) int64

func Register(key int32, handle verifyConditionHandle) {
	verifyCondition[key] = handle
}

// Condition 数组形式条件
type Condition []int32

func (c Condition) GetCondition() (r int32) {
	if len(c) > 0 {
		r = c[0]
	}
	return
}

func (c Condition) GetKey() (r int32) {
	if len(c) > 1 {
		r = c[1]
	}
	return
}
func (c Condition) GetGoal() (r int32) {
	if len(c) > 2 {
		r = c[2]
	}
	return
}
func (c Condition) GetArgs() (r []int32) {
	if len(c) > 3 {
		r = append(r, c[3:]...)
	}
	return
}
