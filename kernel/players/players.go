package players

import "github.com/hwcer/yyds/kernel/players/player"

type Players interface {
	Try(uid uint64, handle player.Handle) error
	Get(uid uint64, handle player.Handle) error
	Load(uid uint64, init bool, handle player.Handle) (err error)
	Range(func(uint64, *player.Player) bool)
	Store(uint64, *player.Player) //存储玩家对象，用于初始化
	Delete(uint64)
	Locker(uid []uint64, handle player.LockerHandle, done ...func()) error
}
