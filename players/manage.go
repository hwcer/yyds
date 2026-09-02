package players

import (
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/player"
)

// ============================================================================
// 玩家容器:管理器 + 单玩家取用
// ============================================================================
//
// 并发模型只有一种:每玩家一把互斥锁(Player.Lock/Unlock),业务跑在调用方(rpcx 请求)协程上,
// 同一玩家串行、不同玩家真并行。跨玩家一律走批量锁(batch.go)。
//
// manage 与 w 都由 newManage 一次性建好、之后只读,所以 **manage == nil ⇔ 本进程从没启动过
// 玩家系统** —— service.go 的可服务性判定就靠这条,不需要另立标记。

var (
	//manage 玩家管理器,nil 表示本进程没有玩家容器(见 service.go)
	manage *player.Manage
	//w 批量锁队列
	w *await.Await
)

// 批量锁队列的两个参数。**刻意不做成配置项** —— 它们不是需要按项目调的旋钮:
//
//	batchCap      纯突发吸收能力,调大无风险也无收益。投递是阻塞式(await.Sync 的 c <- msg),
//	              满了卡住的是投递方,而投递方到这里时已经让出了自己的玩家锁,不持有任何东西
//	batchTimeout  「愿意在**队列里**排多久」,不是「任务最多跑多久」:await.Message.Wait 的
//	              CAS 失败(handler 已开跑)会 Reset 计时器继续等,跑起来的任务不会被打断。
//	              所以超时 ⇔ 任务**一次都没执行**。调大只是把「快速失败」换成「客户端干等」
//
// 🔴 真正想提吞吐的人会盯上的那个旋钮 —— worker 数 —— 恰恰**不是选项**:
// await 的单 worker 就是批量锁的防 ABBA 机制本身,见 newManage。
const (
	batchCap     = 128
	batchTimeout = time.Second * 5
)

// newManage 创建玩家容器
//
// 🔴 **await 只有一个 worker,这不是疏忽而是防死锁机制本身**:批量锁按传入顺序逐个抢锁,
// 两个批量锁并发、成员集合交叉且顺序相反就是 ABBA。全服所有批量锁排成一队,环就构不成。
// 所以**不能靠加 worker 提吞吐**(要提就得改成按 uid 排序取锁,那是另一套设计)。
func newManage() {
	w = await.New(batchCap, batchTimeout)
	manage = player.NewManage()
}

// get 只获取在线玩家。公开入口见 Get
func get(uid string, handle player.Handle) error {
	p, ok := manage.Load(uid)
	if !ok {
		return errors.ErrNotOnline
	}
	p.Lock()
	defer p.Unlock()
	//必须先判状态再 Reset:拿到锁时可能已经被 released 销毁。
	//Updater==nil 是"只占锁位"留下的空壳(见 load 的 init=false 分支):它按定义就不是在线玩家,
	//这里必须挡住,否则会把 nil Updater 交给业务。返回 ErrNotOnline 也正好让 context/locker.go
	//与 gomcp 那两条「Get 失败→退回 Load」的降级链继续成立,由 load 的 Loading 去自愈它。
	if p.Denied(atomic.LoadInt32(&p.Status)) || p.Updater == nil {
		return errors.ErrNotOnline
	}
	p.Reset()
	defer p.Release()
	return handle(p)
}

// load 加载玩家;init=false 时只占锁位不加载数据。公开入口见 Load / Login
func load(uid string, test, init bool, handle player.Handle) (err error) {
	r := player.New(uid, test)
	r.Lock()
	i, loaded := manage.LoadOrStore(r.Key(), r)
	if loaded {
		r.Unlock()
		r = i
		r.Lock()
	} else if !init {
		//空壳必须补一次心跳:player.New 不设 heartbeat(恒 0),而 daemon 按
		//Heartbeat()<offlineTime 判回收、内存压力下又按 heartbeat 升序决定先放谁——
		//不刷的话它是全场最先被释放的对象,等不到有人来自愈。preload 同款做法。
		r.KeepAlive(0)
	}
	defer r.Unlock()
	if !init {
		//只占锁位:不 Loading。空壳留在管理器里等人自愈,不删不打拒绝态——处理期间别人
		//可能已经拿到同一个指针正在等锁,删掉等于让它在管理器之外操作孤儿对象。
		//Reset/Release 仍照做(对空壳是空操作):玩家本就在内存时少了这一步,会给出一个
		//"非 nil 但 now 是零值"的 Updater——能用、不报错、时间全错。
		//
		//📌 这里判的**不是**"在不在线":Denied 只含 Locked/Released/Terminated
		//(见 player/status.go),离线的 None/Disconnect/Offline 一律放行 ——
		//本方法对离线玩家本来就是可用的。误导人的是错误码叫 ErrNotOnline,
		//它的实际语义是"这个对象暂时不能操作(正在加载 / 已销毁 / 已被踢)"。
		if r.Denied() {
			return errors.ErrNotOnline
		}
		r.Reset()
		defer r.Release()
		return handle(r)
	}
	if err = r.Loading(test); err != nil {
		//加载失败时 Loading 会把状态置成 StatusReleased,而 daemon 只回收
		//None/Offline/Terminated —— 不摘出去的话这个对象会永远留在管理器里且恒为拒绝态,
		//该玩家从此登录不进来,要等进程重启才恢复。
		manage.Delete(r.Key())
		return
	}
	r.Reset()
	defer r.Release()
	return handle(r)
}
