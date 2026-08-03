package verify

import "github.com/hwcer/updater"

// middleware 在 updater 提交前自动验证所有已注册的 Target 条件
type middleware struct {
	dict []Target
}

func (this *middleware) Emit(u *updater.Updater, t updater.EventType) bool {
	//Release 每次请求结束必定触发（无论成功失败），在这里注销自己。
	//请求在 Auto 之后、Submit 之前中断时（如扣道具不足直接置 u.Error），
	//本中间件走不到下面的 Submit 分支，若不在此处返回 false 就会残留到
	//同一 Updater 的下一个请求，把毫不相干的请求判成"条件未达成"。
	if t == updater.EventTypeRelease {
		return false
	}
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
