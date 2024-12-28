package verify

const (
	CompareGte = 0
	CompareLte = 1
)

// Value 根据条件，获取对应计数
type Value interface {
	GetKey() int32 //参数,daily id,item id ....
	GetArgs() []int32
	GetCondition() int32 //条件
}

// Target 验证记数是否达到GetValue值
type Target interface {
	Value
	GetGoal() int32 //任务达成目标
}

// GetVal 获取即时任务当前进度
type GetVal interface {
	GetVal() int64
}

type GetTimes interface {
	GetTimes() [2]int64 //[记数开始时间,记数结束时间]
}

// GetCompare 记数比较方式，默认 大于等于
type GetCompare interface {
	GetCompare() int32
}
