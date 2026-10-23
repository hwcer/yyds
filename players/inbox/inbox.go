// Package inbox 系统事件的收件箱（注册入口本包 On、投递点 players.Emit、
// 心跳消费 Trigger——players daemon 随 migrateTask 任务包在执行池内调用）：
// 事件投递进玩家收件箱，daemon 每 tick 开箱处理（原 async 命名弃用——Go 讨论中的
// 关键字风险 + 通用词无域信息）。
//
// # 执行时机
//
// players.Emit 发射系统事件时：同步监听（players.On 注册）立即执行（u.Emit）；
// 有异步监听（本包 On 注册）的事件当场缓存进 Updater.Cache 并置玩家标记，daemon
// worker 每 tick 扫描发现标记的玩家，经状态迁移执行池进玩家协程执行 Tick
// （合成请求：Lock→Reset→派发→Submit→Release）。
//
// 收件监听里再 players.Emit 的新事件当场缓存并重新置位，下个 tick 继续——任务链跨
// tick 收敛。执行池投满时标记仍在，下个 tick 重新收集（与 disconnect 的「丢掉不丢
// 状态」同机制）；玩家已释放（Updater 置空）时缓存随实例消亡，下次登录重发事件自愈。
//
// # CD 门闸（Func 返回值语义：按监听隔离、闸住丢弃、新事件触发）
//
// 门闸的归属是**事件 × 监听 × 玩家**：同一事件的不同监听各管各的 CD，互不干扰——
// A 要 60s、B 要 3600s 时各自按各自节奏触发，无 CD 的监听（返回 0）永不被别人的
// 门闸挡住。返回 0 = 无 CD，每次都执行；返回 T>0 = **本监听**下次允许执行的绝对
// 时刻（unix 秒，基准 u.Unix()）。
//
// 闸住的监听对当条事件就是**丢弃**（不缓存不补跑），CD 结束后再收到事件才会触发。
// CD 是频率控制不是重试队列——消费方要「到期后再试」的，靠事件源再次发射（如登录
// 事件下次登录必发）。同批同事件多条：首条执行设新门闸后，后续条目对该监听同样
// 丢弃。时刻基准 u.Unix()（Trigger 的 Reset 刷新）。
//
// # 缓存与门闸解耦：Emit 无脑缓存，flush 无脑清空
//
// Emit（缓存点）只判「该事件有没有异步监听」，有就入列置标记，不做任何门闸判断
// ——门闸是**监听**的事，分发时才管。flush（分发点）先摘走并清空整个列表，再逐条
// 逐监听过各自门闸：闸住的跳过、到期的执行并存自己的 next。列表寿命至多一个 tick，
// 不随 CD 时间堆积；被闸事件到下个 tick 就出清，届时监听若仍在闸内则丢弃该条。
//
// # 状态载体：Updater.Cache
//
// 每玩家一份（待处理记录 + 各监听门闸）挂 Updater.Cache，随玩家实例存活、跨请求
// 存活、进程重启即丢。缓存在玩家协程（Emit 点恒持玩家锁）、派发在执行池锁内，
// 不加锁；待处理标记是对象上的原子量，daemon 扫描无锁读。handler 后台执行没有
// 请求上下文：不推 S2CUpdate，改动由尾随 Submit 统一落库，客户端靠下次登录/拉取
// 看见。事件可能重复（登录链路跨天重跑等），handler 必须幂等。
//
// # 事件 id 归业务层
//
// 机制对任意 updater.EventType 开放：框架只保留 -100 段（players/events.go 的玩家
// 生命周期），业务自定义系统事件在自定义段（如 -200）自行定义常量。
package inbox

import (
	"sync/atomic"

	"github.com/hwcer/cosgo/phase"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
)

// Func 异步系统事件监听。返回 0 = 无 CD；返回 T>0 = **本监听**下次允许执行的
// 绝对时刻（unix 秒，基准 u.Unix()）。门闸按事件×监听×玩家隔离,同事件多个监听
// 各管各的 CD,互不干扰。闸住丢弃、新事件触发,语义见包注释。
type Func func(u *updater.Updater, vs ...int32) int64

// listener 注册期包装:idx 是事件内稳定下标(handles 启动期封板后只读),
// 作为门闸 nexts 的 key 组成部分。
type listener struct {
	fn  Func
	idx int32
}

// handles 异步监听注册表：仅启动期写入（On），运行期只读 ⇒ 无锁。
var handles = map[updater.EventType][]listener{}

// On 注册异步系统事件监听：发射时只缓存不执行，由 players daemon 心跳经执行池
// 进玩家协程执行。仅启动期调用（运行期注册与派发竞争，phase 封板后拒绝）；
// 与同步监听 players.On 可并存于同一事件。
func On(t updater.EventType, handle Func) {
	if phase.Sealed() {
		phase.Alert("players/inbox.On(%v)", t)
		return
	}
	handles[t] = append(handles[t], listener{fn: handle, idx: int32(len(handles[t]))})
}

// cacheKey 状态在 Updater.Cache 里的键名。
const cacheKey = "players_inbox"

// event 待处理记录的最小载荷。
type event struct {
	t  updater.EventType
	vs []int32
}

// gateKey 门闸键：事件 × 监听下标。同事件各监听各管各的 CD。
type gateKey struct {
	t   updater.EventType
	idx int32
}

// state 每玩家的异步事件状态：待 tick 处理的记录 + 各监听的个人门闸。
type state struct {
	events []event
	nexts  map[gateKey]int64 // 门闸：事件×监听 → 下次允许执行时刻（缺省 0=无）
}

// Inbox 每玩家的系统事件收件箱（Player.Inbox 字段，createComponents 时创建）。
type Inbox struct {
	u    *updater.Updater
	flag atomic.Bool //有待 tick 处理的记录：Emit 置位，flush 先清后按需重置
}

func New(u *updater.Updater) *Inbox {
	return &Inbox{u: u}
}

// Pending 有待 tick 处理的记录。daemon worker 扫描用，无锁原子读。
func (a *Inbox) Pending() bool {
	return a.flag.Load()
}

// Emit 缓存事件（players.Emit 调用，须持玩家锁）。
// 🔴 无脑缓存:只判「该事件有没有异步监听」,有就入列置标记——不做任何门闸判断,
// 门闸是监听的事,分发时(flush)才管。无异步监听的事件零开销直接返回。
func (a *Inbox) Emit(t updater.EventType, args []int32) {
	if len(handles[t]) == 0 {
		return
	}
	u := a.u
	if u == nil {
		return //空壳/已释放不产生事件
	}
	s := u.Cache.LoadOrCreate(u, cacheKey, func(*updater.Updater) any {
		return &state{nexts: map[gateKey]int64{}}
	}).(*state)
	s.events = append(s.events, event{t: t, vs: append([]int32(nil), args...)})
	a.flag.Store(true)
}

// flush 处理全部待处理记录（Tick 在玩家锁内调用）。**先清标记再无脑清空列表**:
// 摘走 events 后逐条逐监听过各自门闸——闸住的跳过（对当条事件即丢弃,新事件再
// 触发）,到期的执行并存自己的 next;监听里嵌套 players.Emit 的新缓存重新置位,
// 进下一轮（下个 tick）。
func (a *Inbox) flush() {
	u := a.u
	if u == nil {
		return
	}
	a.flag.Store(false)
	s, _ := u.Cache.Get(cacheKey).(*state)
	if s == nil || len(s.events) == 0 {
		return
	}
	var uid string
	if e := u.Entity(); e != nil {
		uid = e.Id()
	}
	events := s.events
	s.events = nil
	for _, ev := range events {
		for _, l := range handles[ev.t] {
			g := gateKey{t: ev.t, idx: l.idx}
			if u.Unix() < s.nexts[g] {
				continue //本监听闸内:丢弃该条,CD 结束后靠新事件再触发
			}
			if at := invoke(l.fn, u, uid, ev.t, ev.vs); at > s.nexts[g] {
				s.nexts[g] = at
			}
		}
	}
}

// Trigger 心跳期处理入口（players daemon 随 migrateTask 任务包在玩家锁内调用，经闭包
// 适配 migrateFunc 形态——对象已持有 Updater，无需玩家参数）：无待处理则本 tick 直接
// 返回（不 Reset 不 Submit 的轻量路径）；有待处理走全套：Reset 刷新 u.now（CD 时钟
// 基准）→ flush 派发 → 尾随 Submit 统一落库（不推 S2CUpdate，客户端靠下次登录/拉取
// 看见）。
func (a *Inbox) Trigger() {
	if a.u == nil {
		return //空壳/已释放：缓存随实例消亡，等下次登录
	}
	if !a.Pending() {
		return
	}
	a.u.Reset()
	a.flush()
	if _, err := a.u.Submit(); err != nil {
		logger.Debug("players inbox trigger uid:%v err:%v", a.u.Entity().Id(), err)
	}
}

// invoke 独立 recover 执行单个监听；panic 按「无 CD」处理（不闸死误伤后续重试）。
func invoke(h Func, u *updater.Updater, uid string, id updater.EventType, vs []int32) (next int64) {
	defer func() {
		if e := recover(); e != nil {
			logger.Alert("players inbox panic event:%v uid:%v err:%v", id, uid, e)
			next = 0
		}
	}()
	return h(u, vs...)
}
