// Package inbox 系统事件的收件箱（注册入口本包 On、投递点 players.Emit、
// 心跳消费 Trigger——players daemon 随 migrateTask 任务包在执行池内调用）：
// 事件投递进玩家收件箱，daemon 每 tick 开箱处理（原 async 命名弃用——Go 讨论中的
// 关键字风险 + 通用词无域信息）。
//
// # 执行时机
//
// players.Emit 发射系统事件时：同步监听（players.On 注册）立即执行（u.Emit）；
// 有异步监听（本包 On 注册）的事件当场记录进 Updater.Cache 并置玩家标记，daemon
// worker 每 tick 扫描发现标记的玩家，经状态迁移执行池进玩家协程执行 Tick
// （合成请求：Lock→Reset→派发→Submit→Release）。
//
// 收件监听里再 players.Emit 的新事件当场投递并重新置位，下个 tick 继续——任务链跨
// tick 收敛。执行池投满时标记仍在，下个 tick 重新收集（与 disconnect 的「丢掉不丢
// 状态」同机制）；玩家已释放（Updater 置空）时记录随实例消亡，下次登录重发事件自愈。
//
// # CD 门闸（Func 返回值语义：闸住保留、心跳放行）
//
// 返回 0 = 无 CD，每次事件都执行；返回 T>0 = 个人重试门闸：T 之前该事件不再投递给
// 该玩家。**闸住的记录不丢**——留在 Cache，daemon 每 tick 复查，u.Unix() ≥ T 即执行：
// 重试时点 = CD 到期 + ≤1 心跳周期（Options.Heartbeat，默认 5s），不等下一次事件。
// 同批同事件多条：首条执行设新门闸后，后续条目继续闸住保留，逐窗放行——消费方必须
// 幂等。门闸按事件×玩家隔离（nexts 以事件为 key，同事件的多个监听共享门闸，都设 T
// 时取更严/大的）。时刻基准 u.Unix()（Trigger 的 Reset 刷新）。
//
// # 状态载体：Updater.Cache
//
// 每玩家一份（待处理记录 + 个人门闸）挂 Updater.Cache，随玩家实例存活、跨请求存活、
// 进程重启即丢。记录点在玩家协程（Emit 发射点恒持玩家锁）、派发点在执行池锁内，不加锁；
// 待处理标记是对象上的原子量，daemon 扫描无锁读。handler 后台执行没有请求上下文：
// 不推 S2CUpdate，改动由尾随 Submit 统一落库，客户端靠下次登录/拉取看见。
// 事件可能重复（登录链路跨天重跑等），handler 必须幂等。
//
// # 事件 id 归业务层
//
// 机制对任意 updater.EventType 开放：框架只保留 -100 段（players/events.go 的玩家
// 生命周期），业务自定义系统事件在自定义段（如 -200）自行定义常量。
package inbox

import (
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/phase"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
)

// Func 异步系统事件监听。返回 0 = 无 CD；返回 T>0 = 个人重试门闸
// （闸住保留、心跳放行，语义见包注释）。
type Func func(u *updater.Updater, vs ...int32) int64

// handles 异步监听注册表：仅启动期写入（On），运行期只读 ⇒ 无锁。
var handles = map[updater.EventType][]Func{}

// On 注册异步系统事件监听：发射时只记录不执行，由 players daemon 心跳经执行池
// 进玩家协程执行。仅启动期调用（运行期注册与派发竞争，phase 封板后拒绝）；
// 与同步监听 players.On 可并存于同一事件。
func On(t updater.EventType, handle Func) {
	if phase.Sealed() {
		phase.Alert("players/inbox.On(%v)", t)
		return
	}
	handles[t] = append(handles[t], handle)
}

// cacheKey 状态在 Updater.Cache 里的键名。
const cacheKey = "players_inbox"

// event 待处理记录的最小载荷。
type event struct {
	t  updater.EventType
	vs []int32
}

// state 每玩家的异步事件状态：待心跳处理的记录 + 个人门闸。
type state struct {
	events []event
	nexts  map[updater.EventType]int64 // 个人门闸：事件 → 下次允许投递的最早时刻（缺省 0=无）
}

// Inbox 每玩家的系统事件收件箱（Player.Inbox 字段，createComponents 时创建）。
type Inbox struct {
	u    *updater.Updater
	flag atomic.Bool //有待心跳处理的记录：Record 置位，Flush 先清后按需重置
}

func New(u *updater.Updater) *Inbox {
	return &Inbox{u: u}
}

// Pending 有待心跳处理的记录。daemon worker 扫描用，无锁原子读。
func (a *Inbox) Pending() bool {
	return a.flag.Load()
}

// Record 记录事件（players.Emit 调用，须持玩家锁）。
// 无异步监听的事件零开销直接返回。
func (a *Inbox) Record(t updater.EventType, args []int32) {
	if len(handles[t]) == 0 {
		return
	}
	u := a.u
	if u == nil {
		return //空壳/已释放不产生事件
	}
	s := u.Cache.LoadOrCreate(u, cacheKey, func(*updater.Updater) any {
		return &state{nexts: map[updater.EventType]int64{}}
	}).(*state)
	s.events = append(s.events, event{t: t, vs: append([]int32(nil), args...)})
	a.flag.Store(true)
}

// due 是否存在到期记录（time.Now 对 nexts 窥探，与 nexts 同为墙钟：nexts 由监听以
// u.Unix()+T 写入，门闸窗秒级，毫秒偏差无意义）。全未到期则本 tick 不做重活——
// Trigger 据此跳过 Reset/Submit，门闸期内每 tick 只剩一次轻量复查。
func (a *Inbox) due() bool {
	u := a.u
	if u == nil {
		return false
	}
	s, _ := u.Cache.Get(cacheKey).(*state)
	if s == nil {
		return false
	}
	now := time.Now().Unix()
	for _, ev := range s.events {
		if now >= s.nexts[ev.t] {
			return true
		}
	}
	return false
}

// flush 处理全部到期记录（Tick 在玩家锁内调用）。**先清标记再排空**：门闸未到的
// 记录保留回列表并重新置位，下个 tick 复查放行；监听里嵌套 players.Emit 的新记录
// 同样重新置位，进下一轮（下个 tick）。
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
	var gated []event
	for _, ev := range events {
		//门闸未到：保留待下个 tick 复查（闸住不丢，心跳放行）
		if u.Unix() < s.nexts[ev.t] {
			gated = append(gated, ev)
			continue
		}
		for _, h := range handles[ev.t] {
			if at := invoke(h, u, uid, ev.t, ev.vs); at > s.nexts[ev.t] {
				//多个监听都设 CD 时取更严（大）的 T
				s.nexts[ev.t] = at
			}
		}
	}
	if len(gated) > 0 {
		s.events = append(s.events, gated...)
		a.flag.Store(true) //闸住保留：标记保持，下个 tick 复查
	}
}

// Trigger 心跳期处理入口（players daemon 随 migrateTask 任务包在玩家锁内调用，经闭包
// 适配 migrateFunc 形态——对象已持有 Updater，无需玩家参数）：门闸窥探，全未到期则本 tick
// 直接返回（不 Reset 不 Submit，门闸期内每 tick 只剩一次轻量复查）；到期才走全套：
// Reset 刷新 u.now（CD 时钟基准）→ flush 派发 → 尾随 Submit 统一落库（不推
// S2CUpdate，客户端靠下次登录/拉取看见）。
func (a *Inbox) Trigger() {
	if a.u == nil {
		return //空壳/已释放：记录随实例消亡，等下次登录
	}
	if !a.due() {
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
