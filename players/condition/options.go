package condition

import (
	"errors"

	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/updater"
)

// 哨兵错误 ErrGoalNotAchieved 已上移至 yyds/errors(显式编号 130);
// 原 ErrTargetMethodNotFound 在 handleNone 重写后已无引用,随之移除。

// Options 全局配置，使用前需设置 Count 函数以支持 Weekly/History 类型的统计查询
var Options = &struct {
	Count func(u *updater.Updater, key int32, start, end *times.Times) (r int64, err error)
}{
	Count: defaultCountFunc,
}

func defaultCountFunc(_ *updater.Updater, _ int32, _, _ *times.Times) (r int64, err error) {
	return 0, errors.New("未设置统计函数，无法使用统计数据")
}
