package condition

import (
	"github.com/hwcer/updater"
)

const updaterPlugName = "_updater_condition_plug"

func New(u *updater.Updater) *Verifier {
	return &Verifier{u: u}
}

// Verifier 全系统统一条件验证实现，挂在 player.Player.Condition 上。
//
// 注意别和 updater.Updater.Verify() 混了：那是提交前的 data→verify 收敛，与条件校验无关。
type Verifier struct {
	u *updater.Updater
}

func (v *Verifier) create(_ *updater.Updater) updater.Middleware {
	return &middleware{}
}

// Auto 自动验证失败时 返回错误,不需要配合Verify使用
func (v *Verifier) Auto(target Target) {
	if target.GetCondition() == TypeNone {
		return
	}
	v.Target(target)
	plug := v.u.Middleware.LoadOrCreate(v.u, updaterPlugName, v.create).(*middleware)
	plug.dict = append(plug.dict, target)
}

// Target 预读数据,手动验证
func (v *Verifier) Target(target Value) {
	switch target.GetCondition() {
	case TypeData:
		v.u.Select(target.GetKey())
	case TypeEvents:
	case TypeMethod:
		if i := GetMethod(target.GetKey()); i != nil {
			i.Target(v.u, target)
		}
	}
}

// Value 查询值
func (v *Verifier) Value(target Value) int64 {
	return value(v.u, target)
}

// Verify 检查Target中加入的所有条件是否符合
// 必须已经使用过 Target
// 必须手动执行过 updater.Data()
func (v *Verifier) Verify(target Target) (err error) {
	if err = v.u.Data(); err != nil {
		return err
	}
	return verify(v.u, target)
}
