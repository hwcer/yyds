package verify

// Compare 目标比较方式
const (
	CompareGte = 0 // 大于等于(默认)
	CompareLte = 1 // 小于等于
)

// Judge ARGS参数判断类型
// 当 Judge > 0 时，获取的数据与ARGS进行比较，匹配成功返回1作为任务进度，否则返回0
// 1-9: 单值比较，使用ARGS[0]作为比较目标
// 10+: 集合比较，使用完整ARGS列表
const (
	JudgeNone     = 0  // 不判断，直接使用原始值
	JudgeEqual    = 1  // 等值：val == ARGS[0]
	JudgeGte      = 2  // 大于等于：val >= ARGS[0]
	JudgeLte      = 3  // 小于等于：val <= ARGS[0]
	JudgeContains = 10 // 包含：val 存在于 ARGS 列表中
	JudgeRange    = 11 // 范围：ARGS[0] <= val <= ARGS[1]
)

// Value 根据条件获取对应计数
type Value interface {
	GetKey() int32       // 数据键，如 daily id, item id 等
	GetCondition() int32 // 条件类型，决定取值方式
}

// Target 验证计数是否达到目标值
type Target interface {
	Value
	GetGoal() int32 // 任务达成目标值
}

// Judge ARGS参数判断方式
// 实现此接口后，value() 会在获取原始值后根据 GetJudge() 类型对 val 与 ARGS 进行比较
type Judge interface {
	GetArgs() []int32 // 附加参数，配合 Judge 使用
	GetJudge() int32  // 判断类型，参见 Judge* 常量
}

// GetVal 获取即时任务当前进度
type GetVal interface {
	GetVal() int64
}

// GetTimes 获取计数时间范围
type GetTimes interface {
	GetTimes() [2]int64 // [开始时间, 结束时间]
}

// Errorf 条件不满足时的自定义错误
type Errorf interface {
	Errorf(v int64) error
}

// GetCompare 目标比较方式，默认大于等于
type GetCompare interface {
	GetCompare() int32
}
