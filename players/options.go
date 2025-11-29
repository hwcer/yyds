package players

import "github.com/hwcer/cosmo"

var (
	Heartbeat = 5 //心跳间隔(S)

	HeartbeatConnectedTime  int64 = 120 //N秒无心跳,假死,视为断开连接
	HeartbeatDisconnectTime int64 = 120 //断开连接N秒触发掉线状态
	HeartbeatOfflineTime    int64 = 60  //掉线状态等待N秒 开始清理
)

type AsyncModel int8

const (
	AsyncModelLocker  AsyncModel = iota //用户锁模式,基于用户层面，并发更高,但用户之间数据交互麻烦，需要使用 NewLocker 同时锁定多个用户
	AsyncModelChannel                   //通道模式,全局通道无并发风险，但并发相对低，容易被高延时接口堵塞
)

var Options = struct {
	Preload       Preload
	AsyncModel    AsyncModel
	MemoryPlayer  int32 //常驻内存的玩家数量
	MemoryRelease int32 //回收站(release)玩家数量达到N时开始清理内存,缓存数量>=MemoryPlayer + MemoryRelease 开始执行清理计划

}{
	MemoryPlayer:  2000,
	MemoryRelease: 100,
}

type Preload interface {
	TX() *cosmo.DB //返回当前数据库操作，设定好排序以及过滤条件
	Limit() int64  // 最大加载玩家数量
}
