package condition

import "github.com/hwcer/updater"

// 条件类型，即 Value.GetCondition() 的取值。
//
// 接口方法仍叫 GetCondition()：配置表生成代码在实现它，改方法名会波及导表链路。
const (
	TypeNone    int32 = 0   //无条件直接返回成功
	TypeData    int32 = 1   //基础数据,日常，成就记录
	TypeEvents  int32 = 2   //即时任务，监听数据,仅限于任务
	TypeMethod  int32 = 9   //需要方法实现
	TypeWeekly  int32 = 101 //周数据,基于daily
	TypeHistory int32 = 102 //历史数据
)

var handles = make(map[int32]handleFunc)

// handleFunc times  开始时间，结束时间仅仅用在 TypeHistory 类型的活动中
type handleFunc func(u *updater.Updater, handle Value) int64

func Register(key int32, handle handleFunc) {
	handles[key] = handle
}

// Array 数组形式条件：[条件类型, 数据键, 目标值]
type Array []int32

func (c Array) GetCondition() (r int32) {
	if len(c) > 0 {
		r = c[0]
	}
	return
}

func (c Array) GetKey() (r int32) {
	if len(c) > 1 {
		r = c[1]
	}
	return
}
func (c Array) GetGoal() (r int32) {
	if len(c) > 2 {
		r = c[2]
	}
	return
}

//func (c Array) GetArgs() (r []int32) {
//	if len(c) > 3 {
//		r = append(r, c[3:]...)
//	}
//	return
//}
