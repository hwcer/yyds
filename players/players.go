package players

import "github.com/hwcer/yyds/players/player"

type Players interface {
	Get(uid string, handle player.Handle) error                         //仅获取在线玩家
	Load(uid string, test, init bool, handle player.Handle) (err error) // get or load;init=false 只占锁位不加载数据
	Range(func(string, *player.Player) bool)
	Store(string, *player.Player) //存储玩家对象，用于初始化
	Delete(string)
	Locker(self string, uid []string, args any, handle player.LockerHandle, done ...func()) (any, error)
}
