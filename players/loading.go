package players

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/yyds/players/player"
)

var Preload func(limit int, callback loadingHandler)

type loadingHandler func(uid uint64, name string) (next bool)

// loading 初始加载用户到内存
func loading() (err error) {
	if Options.MemoryInstall == 0 {
		return nil
	}
	if Preload == nil {
		logger.Alert("未设置预加载函数 players.Preload,放弃预加载")
		return nil
	}
	var n int
	Preload(Options.MemoryInstall, func(uid uint64, name string) (next bool) {
		p := player.New(uid)
		if err = p.Loading(true); err == nil {
			n += 1
			ps.Store(uid, p)
			p.KeepAlive(times.Unix())
			logger.Debug("预加载用户: [%v] %v", uid, name)
		}
		return true
	})

	logger.Trace("累计预加载用户数量:%v\n", n)
	return
}
