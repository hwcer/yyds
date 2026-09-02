package players

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/players/player"
)

// ============================================================================
// 状态迁移执行池:daemon 只扫描收集,所有会抢玩家锁的动作都在这里跑
// ============================================================================
//
// daemon 是**单协程**,而三种迁移都要抢玩家锁,并且都可能很慢:
//
//	disconnect / offline  持锁 Emit 业务事件(离线结算之类,快慢由业务决定)
//	released              持锁 Reset + Destroy,而 Destroy 里是一次真实的 BulkWrite 落库
//
// 串行跑的话,一次退潮 N 个玩家就是 N 次「等锁 + DB 往返」首尾相接,daemon 的扫描周期直接被
// 最慢的那个玩家决定 —— 心跳判定、掉线判定、回收判定全部顺延。所以 daemon 只负责判断
// "谁该迁移到哪个状态",迁移动作本身丢进这里。
//
// 三条性质让这件事是安全的,缺一条都不能这么改:
//
//	CAS 守卫  disconnect / offline / released 第一件事都是 CAS 翻状态,重复投递是空操作
//	互不相干  玩家之间彼此独立,谁先谁后不影响结果
//	可重试    没投出去、或者执行失败的,状态不变,下一个 tick 的扫描会重新收集到

// migrateQueue 队列长度。满了就**放弃投递**而不是阻塞 daemon —— 阻塞就违背了本池存在的意义。
// 丢掉的任务不会丢状态:那些玩家的状态没变,下一个 tick 会重新收集到同一批人。
// 所以它只是突发吸收能力,不需要做成配置项;真正该调的是并发度 Options.MigrateWorker。
const migrateQueue = 1024

var migrate chan func()

// migrateWaitGroup 用来等执行池排空退出,见 migrateWait
var migrateWaitGroup sync.WaitGroup

// migrateWorker 并发度,零值兜底成 4。见 Options.MigrateWorker
func migrateWorker() int {
	if n := Options.MigrateWorker; n > 0 {
		return n
	}
	return 4
}

// migrateStart 由 daemon 在进入扫描循环之前调用一次
func migrateStart() {
	migrate = make(chan func(), migrateQueue)
	for i := 0; i < migrateWorker(); i++ {
		migrateWaitGroup.Add(1) //必须在 CGO 之前 Add,否则 migrateWait 可能看到计数 0 提前返回
		scc.CGO(migrateProcess)
	}
}

// migrateWait 等执行池把队列里剩下的任务全部做完并退出
//
// 停服流程必须先等这一步:那批任务正持着玩家锁在补下线事件、在落库,不等的话 shutdown 的
// 全量释放会跟它们抢同一批玩家 —— CAS 保证不会重复释放,但"未能释放"的计数会把
// 「正被执行池释放中」的人算成失败,停服日志就不可信了。
//
// 没起过 daemon(单元测试 / 无玩家容器的服务)时计数为 0,立即返回。
func migrateWait() {
	migrateWaitGroup.Wait()
}

// migratePost 非阻塞投递;队列满返回 false,调用方只需记一笔,下个 tick 自然重来
func migratePost(f func()) bool {
	if migrate == nil {
		return false //没起 daemon(单元测试 / 无玩家容器的服务)
	}
	select {
	case migrate <- f:
		return true
	default:
		return false
	}
}

// migrateDrain 排空队列并就地执行,**停服专用**,必须在 migrateWait 之后调用
//
// 🔴 有了 migrateProcess 的排空为什么还要它:daemon 与执行池共用同一个 ctx。ctx 触发时
// daemon 可能正好在 worker() 里投递,而池的 worker 已经排空退出了 —— 那批任务谁都不会碰,
// 而且悄无声息。
//
// shutdown 的全量释放本身是它们的超集(数据不会丢),但把「队列里的任务要么被池执行、
// 要么被 shutdown 执行」做成结构保证,比依赖那层超集关系更稳:欠着的下线事件也一并补上了。
//
// migrate 为 nil(没起过 daemon)时 select 直接走 default 返回,不会阻塞。
func migrateDrain() {
	for {
		select {
		case f := <-migrate:
			migrateCall(f)
		default:
			return
		}
	}
}

func migrateProcess(ctx context.Context) {
	defer migrateWaitGroup.Done()
	for {
		select {
		case f := <-migrate:
			migrateCall(f)
		case <-ctx.Done():
			//🔴 收到停服信号**不能直接退**:队列里排着的都是已经判定过、正等着执行的状态迁移 ——
			//disconnect/offline 欠着业务的下线事件(离线结算之类),released 欠着一次 BulkWrite。
			//直接 return 等于把这些连同数据一起丢掉,而且悄无声息。排空再退。
			//
			//能排空是有前提的,三条缺一不可:daemon 与本池共用同一个 ctx,它退出后不再投新任务;
			//任务本身不会派生新任务;队列有界。所以队列只减不增,这个循环必然终止。
			migrateDrain()
			return
		}
	}
}

// migrateCall 逐个 recover
//
// 🔴 不能只在池的循环外面 recover:一个玩家的 Destroy panic 会把整个 worker 协程带走,
// 剩下的玩家从此没人释放。原先这些动作跑在 daemon 里,由 worker() 那层 recover 兜着,
// 代价是**当轮剩下的迁移全部作废**;搬进池子之后按任务隔离,一个坏玩家只影响他自己。
func migrateCall(f func()) {
	defer func() {
		if e := recover(); e != nil {
			logger.Debug("Players migrate error:%v \n %v", e, string(debug.Stack()))
		}
	}()
	f()
}

// releaseAll 并行释放并等到全部完成,返回**没能释放成功**的数量。**停服专用**
//
// 不能借用上面那个执行池:shutdown 由 daemon 的 defer 触发,此时 scc 的 ctx 已经 Done,
// 池里的 worker 正在退出甚至已经退出,投进去的任务没人保证会被执行。
// 而停服的要求是"全部落库",所以这里自己起协程并等齐。
func releaseAll(dict []*player.Player) (failed int32) {
	if len(dict) == 0 {
		return
	}
	n := migrateWorker()
	if n > len(dict) {
		n = len(dict)
	}
	c := make(chan *player.Player)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range c {
				//panic 也算失败:那个玩家的数据没保存成功,得计进去
				ok := false
				migrateCall(func() { ok = released(p) })
				if !ok {
					atomic.AddInt32(&failed, 1)
				}
			}
		}()
	}
	for _, p := range dict {
		c <- p
	}
	close(c)
	wg.Wait()
	return
}
