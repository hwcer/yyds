package players

import "github.com/hwcer/yyds/players/player"

type Players interface {
	Get(uid string, handle player.Handle) error                   //仅获取在线玩家
	Load(uid string, init bool, handle player.Handle) (err error) // get or load
	Range(func(string, *player.Player) bool)
	Store(string, *player.Player) //存储玩家对象，用于初始化
	Delete(string)
	Locker(uid []string, handle player.LockerHandle, args any, done ...func()) (any, error)
}
