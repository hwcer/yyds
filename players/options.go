package players

import (
	"time"

	"github.com/hwcer/cosmo"
)

var ()

type AsyncModel int8

const (
	AsyncModelLocker  AsyncModel = iota //用户锁模式,基于用户层面，并发更高,但用户之间数据交互麻烦，需要使用 NewLocker 同时锁定多个用户
	AsyncModelActor                     //Actor模式,每玩家独立通道，不同玩家并发，同一玩家串行
)

var Options = struct {
	Preload        Preload
	AsyncModel     AsyncModel
	MemoryPlayer   int32 //常驻内存的玩家数量
	MemoryRelease  int32 //回收站(release)玩家数量达到N时开始清理内存,缓存数量>=MemoryPlayer + MemoryRelease 开始执行清理计划
	Heartbeat      int64
	ConnectedTime  int64
	DisconnectTime int64
	OfflineTime    int64
	LockerCap      int           //批量锁(Mutex().Lock/Async)的队列深度
	LockerTimeout  time.Duration //批量锁的【排队】超时,见下方说明
}{
	MemoryPlayer:  2000,
	MemoryRelease: 100,

	Heartbeat:      5,   //心跳间隔(S)
	ConnectedTime:  120, //N秒无心跳,假死,视为断开连接
	DisconnectTime: 120, //断开连接N秒触发掉线状态
	OfflineTime:    60,  //掉线状态等待N秒 开始清理

	//批量锁的两个旋钮。🔴 **加 worker 不是选项** —— await 的单 worker 就是批量锁的
	//防 ABBA 机制本身(理由见 players/locker/players.go 的 New),所以这两个是仅有的杠杆。
	//
	//LockerCap   队列深度。投递是阻塞式(await.Sync 的 c <- msg),满了就卡住投递方 ——
	//            对 Async 卡的是它自己的 scc 协程(没人等),对 Lock 卡的是请求协程
	//            (但它已让出自己的玩家锁,不持有任何东西)。纯突发吸收能力,调大无风险。
	//
	//LockerTimeout ⚠ 语义是「愿意在**队列里**排多久」,不是「任务最多跑多久」:
	//            await.Message.Wait 的 CAS 失败(handler 已开跑)会 Reset 计时器继续等,
	//            跑起来的任务不会被打断。所以超时 ⇔ 任务**一次都没执行**。
	//            调大的代价在 Mutex().Lock 那条同步路径:把「快速失败」换成「客户端干等」。
	//            默认维持 5s —— 队列排不上的概率已由 LockerCap 吃掉了。
	LockerCap:     128,
	LockerTimeout: time.Second * 5,
}

type Preload interface {
	TX() *cosmo.DB //返回当前数据库操作，设定好排序以及过滤条件
	Limit() int64  // 最大加载玩家数量
}
