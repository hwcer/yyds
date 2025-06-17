package context

import (
	"github.com/hwcer/yyds/players"
	"github.com/hwcer/yyds/players/player"
)

//多用户批量锁操作,不需要交互的情况下直接注释或者删除此文件

// Locker 批量获取玩家锁
// handle 获取批量操作权限
// next   获取操作结束后是否需要回到玩家自身,

func (this *Context) Locker(uids []string, handle player.LockerHandle, args any, next ...func()) (any, error) {
	var p *player.Player
	var done []func()
	if this.Player != nil {
		p = this.Player
		this.Player = nil
		p.Release()
		if players.Options.AsyncModel == players.AsyncModelLocker {
			p.Unlock()
		}
		done = append(done, func() {
			if players.Options.AsyncModel == players.AsyncModelLocker {
				p.Lock()
			}
			p.Reset()
			this.Player = p
		})
	}
	done = append(done, next...)
	return players.Locker(uids, handle, args, done...)
}
