package verify

import (
	"errors"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/updater"
)

var (
	ErrGoalNotAchieved      = values.Error("goal not achieved")
	ErrTargetMethodNotFound = values.Error("任务数据模型未实现接口(GetVal),即时任务无法通过验证")
)

var Options = &struct {
	Count func(u *updater.Updater, key int32, start, end int64) (r int64, err error)
}{
	Count: defaultCountFunc,
}

func defaultCountFunc(u *updater.Updater, key int32, start, end int64) (r int64, err error) {
	return 0, errors.New("未设置统计函数，无法使用统计数据")
}
