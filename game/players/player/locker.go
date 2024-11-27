package player

type Locker interface {
	Data() error
	Get(uid uint64) *Player
	Range(f func(player *Player) bool)
	Select(keys ...any)
	Verify() error
	Submit() error
}

type LockerHandle func(locker Locker)
