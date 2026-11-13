package players

import (
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/cosgo/uuid"
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
	//必须先判状态再 Reset:拿到锁时可能已经被 released 卸载。
	//!Loaded() 是懒加载的 Updater(批量锁 / Load(lazy=true) 建的,未整档加载,读要 Select+Data、
	//扫描报错)——它不是可当在线玩家交给业务的对象,这里必须挡住。返回 ErrNotOnline 也正好让
	//context/locker.go 与 gomcp 那两条「Get 失败→退回 Load」的降级链继续成立,由 load 转正它。
	//Updater==nil 只剩 acquire 之后、prepare 之前那一瞬(锁内),拿到锁时不会再看到,判它只是防御。
	if p.Denied(atomic.LoadInt32(&p.Status)) || p.Updater == nil || !p.Updater.Loaded() {
		return errors.ErrNotOnline
	}
	p.Reset()
	defer p.Release()
	return handle(p)
}

// load 加载玩家并在锁内执行 handle。公开入口见 Load / Login
//
//	test        只选对象:true 用独立的测试对象(key "~uid",见 player.New),不与真实玩家共用;
//	            是否「只改内存不写库」由业务在 handle 里自己调 p.Updater.Testing(true)
//	lazy=false  整档加载(懒加载实例就地转正,已整档的不重拉)
//	lazy=true   懒加载:不预拉任何数据,读前 Select + Data(契约同批量锁,见 player.Locker)
//
// lazy 与 Player.Loading / updater.Loading 的同名参数同义。
func load(uid string, test, lazy bool, handle player.Handle) (err error) {
	r, err := acquire(uid, test)
	if err != nil {
		return err
	}
	defer r.Unlock()
	if err = prepare(r, lazy); err != nil {
		return err
	}
	//Reset 必做:少了这一步会给出一个"非 nil 但 now 是零值"的 Updater——能用、不报错、时间全错
	r.Reset()
	defer r.Release()
	return handle(r)
}

// acquire 取得 uid 的玩家锁位,返回时**已持有该玩家的锁**(调用方负责 Unlock)。
// 玩家在内存就用现成的(先判拒绝态);不在内存放入一个新对象。数据由 prepare 负责。
// Load / Login 与批量锁共用这一份,两条路「不在内存」的处理从此一致。
func acquire(uid string, test bool) (*player.Player, error) {
	//🔴 uid 合法性必须在这里挡:漏掉的后果不是报错而是**静默**——任何一个字符串都会在 Manage
	//里种下一个对象,占着内存等 daemon 回收,而调用方以为自己操作成功了
	if !uuid.IsValid(uid) {
		return nil, errors.ErrArgEmpty
	}
	r := player.New(uid, test)
	r.Lock()
	if i, loaded := manage.LoadOrStore(r.Key(), r); loaded {
		i.Lock()
		r.Unlock()
		r = i
		//已在内存的也要判拒绝态:Released 的对象 Updater 已被卸载、Terminated 的已被踢。
		//📌 判的**不是**"在不在线":Denied 只含 Locked/Released/Terminated(见 player/status.go),
		//离线的 None/Disconnect/Offline 一律放行。错误码叫 ErrNotOnline 是历史叫法,
		//实际语义是"这个对象暂时不能操作(正在加载 / 已卸载 / 已被踢)"
		if r.Denied() {
			r.Unlock()
			return nil, errors.ErrNotOnline
		}
	} else {
		//新对象必须补一次心跳:player.New 不设 heartbeat(恒 0),而 daemon 按
		//Heartbeat()<offlineTime 判回收、内存压力下又按 heartbeat 升序决定先放谁——
		//不刷的话它是全场最先被释放的对象。preload 同款做法。
		r.KeepAlive(0)
	}
	return r, nil
}

// prepare 保证玩家有可用的 Updater(调用方持锁):lazy=true 至少懒加载(已有 Updater 的不动);
// lazy=false 整档(懒加载实例就地转正,已整档的跳过)。
//
// 失败时对象若落到拒绝态(新建 Updater 失败 / panic,见 Player.Loading)就摘出 Manage:daemon 只回收
// None/Offline/Terminated,不摘的话这个对象恒为拒绝态,该玩家要等进程重启才登录得进来。
// 懒加载实例转正失败时状态已恢复、对象仍可用(updater 退回懒加载),不摘:它可能带着
// 未提交成功的 BulkWrite 队列,摘了就丢了。
// 懒加载玩家**留在 Manage 里不删**:处理期间别人可能已经拿到同一个指针正在等锁,删掉等于让它
// 在 Manage 之外操作孤儿对象。之后登录由 Loading 转正,没人来就由 daemon 回收。
func prepare(r *player.Player, lazy bool) error {
	if r.Updater != nil && (lazy || r.Updater.Loaded()) {
		return nil
	}
	if err := r.Loading(lazy); err != nil {
		if r.Denied() {
			manage.Delete(r.Key())
		}
		return err
	}
	return nil
}
