package verify

import "github.com/hwcer/updater"

// middleware 在 updater 提交前自动验证所有已注册的 Target 条件
type middleware struct {
	dict []Target
}

func (this *middleware) Emit(u *updater.Updater, t updater.EventType) bool {
	if t != updater.EventTypeSubmit {
		return true
	}
	for _, tar := range this.dict {
		if u.Error = verify(u, tar); u.Error != nil {
			return false
		}
	}
	return false
}

func (this *middleware) Release(u *updater.Updater) bool {
	return false
}
