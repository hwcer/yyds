package player

import "sync/atomic"

const (
	StatusNone       int32 = iota //初始仅仅被初始化到内存,启动服务器或者异步操作读取到内存
	StatusLocked                  //临时锁定状态
	StatusConnected               //上线
	StatusDisconnect              //断开连接
	StatusOffline                 //掉线  进入回收队列,此时上线还能抢救一下
	StatusReleased                //正在释放资源,此时无法进行任何操作
	StatusTerminated              //被踢下线,拒绝一切请求,等 daemon 下一个 tick 释放
)

// Denied 是否处于拒绝访问的状态,拒绝态集合只在这里维护
//
//	StatusLocked     数据正在加载,任何读写都不安全
//	StatusReleased   Updater 已销毁,碰数据会 panic
//	StatusTerminated 已被踢下线,等 daemon 释放
func (p *Player) Denied(status ...int32) bool {
	var s int32
	if len(status) > 0 {
		s = status[0]
	} else {
		s = atomic.LoadInt32(&p.Status)
	}
	switch s {
	case StatusLocked, StatusReleased, StatusTerminated:
		return true
	default:
		return false
	}
}
