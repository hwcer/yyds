package players

import "github.com/hwcer/yyds/players/player"

type Players interface {
	Try(uid string, handle player.Handle) error
	Get(uid string, handle player.Handle) error
	Load(uid string, init bool, handle player.Handle) (err error)
	Range(func(string, *player.Player) bool)
	Store(string, *player.Player) //存储玩家对象，用于初始化
	Delete(string)
	Locker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error)
}
