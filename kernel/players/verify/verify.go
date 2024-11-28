package verify

import (
	"github.com/hwcer/updater"
)

const updaterPlugName = "_updater_verify_plug"

func New(u *updater.Updater) *Verify {
	return &Verify{u: u}
}

// Verify 全系统统一验证实现
type Verify struct {
	u *updater.Updater
}

func (v *Verify) create() updater.Middleware {
	return &plugs{}
}

// Auto 自动验证失败时 返回错误,不需要配合Verify使用
func (v *Verify) Auto(target Target) {
	if target.GetCondition() == ConditionNone {
		return
	}
	v.Target(target)
	plug := v.u.Events.LoadOrCreate(updaterPlugName, v.create).(*plugs)
	plug.dict = append(plug.dict, target)
}

// Target 预读数据,手动验证
func (v *Verify) Target(target Target) {
	switch target.GetCondition() {
	case ConditionData:
		v.u.Select(target.GetKey())
	case ConditionEvents:
		//handle := v.u.Handle(model.UpdaterTaskName)
		//handle.Select(target.GetId())
	case ConditionMethod:
		if i := GetMethod(target.GetKey()); i != nil {
			i.Target(v.u, target)
		}
	}
}

// Value 查询值
func (v *Verify) Value(target Value) int64 {
	return value(v.u, target)
}

// Verify 检查Target中加入的所有条件是否符合
// 必须已经使用过 Target
// 必须手动执行过 updater.Data()
func (v *Verify) Verify(target Target) (err error) {
	if err = v.u.Data(); err != nil {
		return err
	}
	return verify(v.u, target)
}
