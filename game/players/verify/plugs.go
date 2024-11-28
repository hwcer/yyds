package verify

import "github.com/hwcer/updater"

type plugs struct {
	dict []Target
}

func (this *plugs) Emit(u *updater.Updater, t updater.EventType) bool {
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
