package players

const (
	Heartbeat = 5 //心跳间隔(S)

	HeartbeatConnectedTime  = 300 //N秒无心跳,假死,视为断开连接
	HeartbeatDisconnectTime = 300 //断开连接N秒触发掉线状态
	HeartbeatOfflineTime    = 600 //掉线状态等待N秒 开始清理
)

type AsyncModel int8

const (
	AsyncModelLocker  AsyncModel = iota //用户锁模式,基于用户层面，并发更高,但用户之间数据交互麻烦，需要使用 NewLocker 同时锁定多个用户
	AsyncModelChannel                   //通道模式,全局通道无并发风险，但并发相对低，容易被高延时接口堵塞
)

var Options = struct {
	AsyncModel AsyncModel
	PreloadMax int64 //启动服务器时预加载数量,0:全部
	PreloadDay int64 //启动服务器时预加载最近N天登录过的,0:全部
	//ReleaseTime   int   //至少间隔N个playersHeartbeat才会执行清理任务
	MemoryPlayer  int32 //常驻内存的玩家数量
	MemoryRelease int32 //回收站(release)玩家数量达到N时开始清理内存,缓存数量>=MemoryPlayer + MemoryRelease 开始执行清理计划

}{
	PreloadMax: 10000,
	PreloadDay: 7,
	//ReleaseTime:   10,
	MemoryPlayer:  10000,
	MemoryRelease: 1000,
}
