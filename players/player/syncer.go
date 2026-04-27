package player

// Syncer 玩家并发控制器接口
type Syncer interface {
	Lock()
	Unlock()
	Close()
}
