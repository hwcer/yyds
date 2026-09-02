package players

import (
	"github.com/hwcer/cosmo"
)

// Options 玩家系统的运行时配置,**必须在 players.Start 之前设定**。
//
// 📌 并发模型只有一种:每玩家一把互斥锁(Player.Lock/Unlock),业务跑在调用方(rpcx 请求)协程上。
// 曾经还有一个 AsyncModelActor(每玩家一条通道、业务在通道里执行),已整体移除 ——
// 原因与"什么时候值得重做"见 players/README.md 的「为什么没有 actor 模式」,加回来之前先读那一节。
var Options = struct {
	//Preload 启动时预加载哪些玩家。不配则不预加载(启动日志里会 Alert 一句)
	Preload Preload

	//MemoryPlayer 常驻内存的玩家数量。缓存总量没超过 MemoryPlayer+MemoryRelease 时
	//daemon 根本不启动回收 —— 离线玩家就留在内存里等重连,重连是零成本的
	MemoryPlayer int32

	//MemoryRelease 回收的**触发余量**:缓存总量 >= MemoryPlayer + MemoryRelease 才开始清理。
	//留这段余量是为了避免在阈值上反复横跳(刚放掉一个又立刻越线)
	MemoryRelease int32

	//ReleaseBatch 一个 tick 释放一批,这是一批的大小。
	//
	//必须分批:每个脏玩家一次 BulkWrite,一次退潮把上千个的落库同时压给数据库不是好主意,
	//状态迁移执行池的队列也吃不下。这一批没轮到的留在回收站,下个 tick 继续。
	//
	//⚠ 零值是**致命**的:一个都放不掉,内存只涨不跌 —— Start 里有兜底,见那里的说明。
	//一批放满了仍有积压时会打一条 Trace,免得从监控上看像是"清不动"。
	ReleaseBatch int32

	//Heartbeat ⚠ 是 **daemon 的扫描间隔**(秒),不是客户端心跳间隔(那个由网关决定),
	//也与 Player.Heartbeat()(最后一次心跳的时间戳)不是一回事。
	//下面三档超时的判定精度就是它 —— 调大等于状态迁移整体变迟钝。
	Heartbeat int64

	//ConnectedTime 在线玩家 N 秒无心跳 → 视为假死,迁 Disconnect,触发 EventDisconnect
	ConnectedTime int64
	//DisconnectTime 断开连接 N 秒 → 迁 Offline,触发 EventOffline
	DisconnectTime int64
	//OfflineTime 掉线 N 秒 → 进回收站,等内存压力触发释放
	//
	//三档串起来:从最后一次心跳到进回收站最少 ConnectedTime+DisconnectTime+OfflineTime 秒,
	//期间随时可以重连,状态直接跳回 Connected
	OfflineTime int64

	//MigrateWorker 玩家状态迁移(disconnect / offline / released)执行池的并发度。
	//
	//daemon 只负责扫描收集,这三个动作都要抢玩家锁,而 released 还会在锁内做一次 BulkWrite
	//落库 —— 全部丢进池里跑,daemon 的扫描周期才不会被最慢的那个玩家决定(见 migrate.go)。
	//
	//瓶颈在**数据库**而不是 CPU:调大能压平退潮时的释放耗时,同时也是压给 mongo 的并发写入量,
	//所以别按核数设,按库扛得住多少并发写来设。停服时的全量落库(releaseAll)用的也是这个值。
	//日志里持续出现"状态迁移队列已满"就是该调大的信号。
	MigrateWorker int
}{
	MemoryPlayer:  2000,
	MemoryRelease: 100,
	ReleaseBatch:  100,

	Heartbeat:      5,   //daemon 扫描间隔
	ConnectedTime:  120, //无心跳多久算假死
	DisconnectTime: 120, //断线多久算掉线
	OfflineTime:    60,  //掉线多久进回收站

	MigrateWorker: 4,
}

// Preload 预加载数据源。不实现则跳过预加载,玩家全部按需加载
type Preload interface {
	TX() *cosmo.DB //返回当前数据库操作，设定好排序以及过滤条件
	Limit() int64  // 最大加载玩家数量
}
