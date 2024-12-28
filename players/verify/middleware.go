package verify

import "github.com/hwcer/updater"

type middleware struct {
	dict []Target
}

func (this *middleware) Emit(u *updater.Updater, t updater.EventType) bool {
	if t != updater.OnPreVerify {
		return true
	}
	for _, tar := range this.dict {
		if u.Error = verify(u, tar); u.Error != nil {
			return false
		}
	}
	return false
}
