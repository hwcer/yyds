package emitter

import (
	"sync"
	"sync/atomic"

	"github.com/hwcer/updater"
)

// 全局业务事件
type EventsFunc func(u *updater.Updater, vs ...int32)

// 全局监听表:写时复制(COW)。
//
// 派发(每次 Emit / 提交期)远多于注册,且在请求热路径上:派发**不加锁**,原子取当前快照;
// 注册在 mutex 下复制一份新 map(被追加的那个事件的切片也复制),再原子发布。
// 已发布的快照永不修改,派发中的协程读旧快照、注册发布新快照互不竞争。
//
// 因此 On 在启动封板后也能调用——典型场景:按配置热更新增的事件监听(活动排行换了计分事件)。
// 监听注册后不移除:不再需要的监听由回调自己判断(如对照当前配置)后直接返回。
var (
	eventsMutex sync.Mutex
	events      atomic.Pointer[map[int32][]EventsFunc] //惰性:提交期随事务派发
	eagerness   atomic.Pointer[map[int32][]EventsFunc] //急切:Emit 当场执行
)

// On 注册惰性监听：经 Player.Emit 触发的事件在 updater 提交期才派发
//
// eager=true 注册急切监听(Emit 当场执行,限只读校验,副作用不随请求回滚)。
// 启动前后均可调用(见上方 COW 说明)。
func On(t int32, handle EventsFunc, eager ...bool) {
	Listen(t, handle, eager...)
}

// Listen 兼容旧名，等同 On。
func Listen(t int32, handle EventsFunc, eager ...bool) {
	if handle == nil {
		return
	}
	table := &events
	if len(eager) > 0 && eager[0] {
		table = &eagerness
	}
	eventsMutex.Lock()
	defer eventsMutex.Unlock()
	var old map[int32][]EventsFunc
	if p := table.Load(); p != nil {
		old = *p
	}
	m := make(map[int32][]EventsFunc, len(old)+1)
	for k, v := range old {
		m[k] = v
	}
	//复制切片再追加:旧快照可能正被派发协程遍历,不能在它的底层数组上 append
	fs := make([]EventsFunc, 0, len(old[t])+1)
	fs = append(fs, old[t]...)
	m[t] = append(fs, handle)
	table.Store(&m)
}

// dispatch 按 eager 标记派发（eagerOnly=true 跑急切监听，false 跑惰性监听）。
func dispatch(u *updater.Updater, t int32, args []int32, eager bool) {
	table := &events
	if eager {
		table = &eagerness
	}
	p := table.Load()
	if p == nil {
		return
	}
	for _, fn := range (*p)[t] {
		fn(u, args...)
	}
}
