package options

const (
	PlayersHeartbeat  = 5    //心跳间隔(S)
	PlayersDisconnect = 300  //N秒无心跳,假死,视为掉线
	PlayersRelease    = 3600 //掉线N秒进入待销毁队列
)

type AsyncModel int8

const (
	AsyncModelLocker  AsyncModel = iota //用户锁模式,基于用户层面，并发更高,但用户之间数据交互麻烦，需要使用 NewLocker 同时锁定多个用户
	AsyncModelChannel                   //通道模式,全局通道无并发风险，但并发相对低，容易被高延时接口堵塞
)

var Options = struct {
	AsyncModel    AsyncModel
	ReleaseTime   int //至少间隔N个playersHeartbeat才会执行清理任务
	MemoryPlayer  int //常驻内存的玩家数量
	MemoryRelease int //回收站(release)玩家数量达到N时开始清理内存,缓存数量>=MemoryPlayer + MemoryRelease 开始执行清理计划
	MemoryInstall int //启动服务器时预加载数量
}{
	ReleaseTime:   10,
	MemoryPlayer:  5000,
	MemoryRelease: 500,
	MemoryInstall: 1000,
}
