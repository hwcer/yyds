package actor

import (
	"sync"

	"github.com/hwcer/yyds/players/player"
)

func newPlayer(uid string, test bool) *player.Player {
	p := player.New(uid, test)
	p.Syncer = NewSyncer()
	return p
}

// ============================================================================
// actor = 全局玩家通道
// ============================================================================
//
// 这是一个**传统编程思想 + Go 语言特性**实现的模型:拿一把全局的"操作权",在里面像写单线程
// 程序一样随便改数据。它的取舍非常极端,先把话说死:
//
//	唯一的好处   一旦取得数据操作权限,就可以修改【任意】玩家的数据 —— 跨角色操作极其方便,
//	             不用成批取锁、不用担心顺序、不可能死锁、也不可能漏锁
//	最大的特点   **把并发绑死在一个协程内** —— 全服的玩家操作串行执行,吞吐就是这一个协程的吞吐
//
// 🔴 **actor 模式没有"每个玩家一条通道"这回事**。整个模式只有**一条全局通道**(w),
// 一切取得玩家的操作都排队进入它,**所有操作也都在它里面跑完**。
// 由此得到 actor 唯一的、也是全部的同步语义:
//
//	排到 = 取得了【所有】玩家的操作权限
//
// 所以在通道内可以直接读写任何玩家、任意多个玩家,不需要再逐个上锁。
//
// (locker 模式相反:每玩家一把互斥锁,真并发,但跨玩家要靠 context.Mutex() 小心地成批取锁,
// 取锁顺序、空壳、ABBA 全要自己盯。两者由 players.Options.AsyncModel 二选一,
// 同一进程只有一个生效。选型就是在"跨玩家写起来简单"和"能吃满多核"之间二选一。)
//
// # 曾经的实现与它带来的两个 bug(2026-09-01 修)
//
// 早先 Syncer 是**每玩家一条 chan + 一个 worker 协程**,`Lock()` 往对方通道里投一个
// 停住 worker 的闭包。两条都是错的:
//
//  1. `Lock()` 把 holding 通道写在**共享字段**上,两个协程同时 Lock 同一玩家时后者覆盖前者,
//     实测 8 个协程能同时"持有"同一个玩家,随后 `close of closed channel` panic;
//  2. 业务 handler 跑在**玩家自己的 worker** 上,于是"A 的请求取 B"与"B 的请求取 A"
//     各占着自己的 worker 等对方 —— 双方永久挂起(无超时)。
//
// 两条的根都是同一个:把同步做成了 per-player。收敛回全局通道之后都不复存在。

// gate 与**不走全局通道**的协程互斥
//
// daemon(回收玩家)、Terminate 等跑在自己的协程里,直接调 Player.Lock/Unlock 而不经过 w。
// 全局通道保证「通道内彼此串行」,但挡不住这些人,所以 Player.Lock 落到这把锁上,
// 而通道内的执行体也持有它(见 invoke) —— 两边由此互斥。
var gate sync.Mutex

// Syncer actor 模式的玩家并发控制器:**不持有任何属于自己的东西**
//
// Lock/Unlock 落在全局 gate 上 —— "取得某个玩家"在 actor 里就是"进入全局通道",
// 与是哪个玩家无关,所以每玩家一个实例只是为了满足 player.Syncer 接口。
type Syncer struct{}

func NewSyncer() player.Syncer {
	return &Syncer{}
}

func (c *Syncer) Lock() {
	gate.Lock()
}

func (c *Syncer) Unlock() {
	gate.Unlock()
}

// Close 无资源可关:没有通道、也没有协程
func (c *Syncer) Close() {}
