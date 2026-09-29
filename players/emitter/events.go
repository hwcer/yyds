package emitter

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
)

var started bool

func init() {
	cosgo.On(cosgo.EventTypStarted, func() error {
		started = true
		return nil
	})
}

// Events 全局事件,必须在init中初始化，禁止动态添加
var Events = events{}

type eventEntry struct {
	fn    EventsFunc
	eager bool //急切监听：随 Player.Emit 当场执行，不等提交期
}

type events map[int32][]eventEntry
type EventsFunc func(u *updater.Updater, vs ...int32)

// On 注册惰性监听：经 Player.Emit 触发的事件在 updater 提交期才派发
//（副作用随事务成败同生共死）。
func (e events) On(t int32, handle EventsFunc) {
	e.register(t, handle, false)
}

// Eager 注册急切监听：Player.Emit 调用返回前**当场执行**——适合请求进行中
// 就要同步结论的场合（如拦单：监听器内 u.Errorf 立刻反映到 Updater.Error，
// 调用方随后检查即可拦下请求）。
//
// ⚠️ 急切监听已执行的副作用不随请求回滚（后续请求失败不会撤销），
// 监听器必须只做校验/只读判定，禁止写玩家数据。
func (e events) Eager(t int32, handle EventsFunc) {
	e.register(t, handle, true)
}

// Listen 兼容旧名，等同 On。
func (e events) Listen(t int32, handle EventsFunc) {
	e.register(t, handle, false)
}

func (e events) register(t int32, handle EventsFunc, eager bool) {
	if started {
		logger.Alert("禁止在程序启动后动态添加全局事件")
	} else {
		e[t] = append(e[t], eventEntry{fn: handle, eager: eager})
	}
}

// Emit 立即派发**全部**监听（急切+惰性）。
// 用于不经 Player 缓冲、直接全局触发的场合（连接/断线等系统事件，没有提交期概念）；
// 随玩家事务走的业务事件请用 Player.Emit——那条链路上急切监听当场执行、
// 惰性监听缓冲到提交期。
func (e events) Emit(u *updater.Updater, t int32, args ...int32) {
	//没有数据的玩家(Load init=false 留下的空壳)不产生事件:框架侧虽然不解引用 u,
	//但业务注册的监听几乎一定会,把 nil 交出去等于让业务崩
	if u == nil {
		return
	}
	e.dispatch(u, t, args, false)
	e.dispatch(u, t, args, true)
}

// dispatch 按 eager 标记派发（eagerOnly=true 跑急切监听，false 跑惰性监听）。
func (e events) dispatch(u *updater.Updater, t int32, args []int32, eagerOnly bool) {
	if len(e[t]) == 0 {
		return
	}
	for _, l := range e[t] {
		if l.eager == eagerOnly {
			l.fn(u, args...)
		}
	}
}
