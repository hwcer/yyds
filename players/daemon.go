package players

import (
	"context"
	"runtime/debug"
	"sort"
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players/emitter"
	"github.com/hwcer/yyds/players/player"
)

// Connected 连线，不包括断线重连等
func Connected(p *player.Player, meta values.Metadata) (err error) {
	status := atomic.LoadInt32(&p.Status)
	//拒绝态一律不许上线。下面的 else 本来也会挡住,这里显式判一次,新增拒绝态时不会漏
	if p.Denied(status) {
		return errors.ErrLoginWaiting
	}

	gateway := uint64(meta.GetInt64(gwcfg.ServiceMetadataGateway))
	if gateway == 0 {
		return errors.New("gateway is empty")
	}

	//上线前先确保数据就绪。Connected 只管"算不算在线",数据一向由 Loading 负责;
	//正常登录路径(Login = Load(init=true) + Connected)早已加载过,Loading 幂等、零成本。
	//这里兜的是 Load(init=false) 留下的空壳被直接激活的情况——没有这一步就会出现
	//"在线却没有数据"的玩家:它在在线数里占一个、Send 也会尝试发包,却要等第一个业务
	//请求经「Get 失败 → 退回 Load(init=true)」降级链才把数据补上。
	//放在 gateway 校验之后:那是纯内存判断,不该为一个注定失败的上线去读一次库。
	if e := p.Loading(false); e != nil {
		logger.Debug("player loading failed on connected:%v,%v", p.Uid(), e)
		return errors.ErrLoginWaiting
	}

	defer func() {
		if err == nil {
			p.KeepAlive(0)
			p.Login = p.Unix()
		}
	}()

	if ip := meta.GetString(gwcfg.ServiceMetadataAddress); ip != "" {
		p.Address = ip
	}

	oldGateway := p.Gateway
	p.Gateway = gateway
	if b := binder.GetBinder(meta, binder.HeaderAccept, binder.HeaderContentType); b != nil {
		p.Binder = b
	}
	// 不同端不同协议顶号
	if status == player.StatusConnected {
		if oldGateway == gateway {
			emitter.Events.Emit(p.Updater, EventReconnect)
		} else {
			emitter.Events.Emit(p.Updater, EventReplace)
		}
		return
	} else if status == player.StatusNone || status == player.StatusDisconnect || status == player.StatusOffline {
		if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusConnected) {
			return errors.ErrLoginWaiting
		}
	} else {
		return errors.ErrLoginWaiting
	}

	if p.Message == nil {
		p.Message = &player.Message{}
	}
	playersOnline.Add(1)
	emitter.Events.Emit(p.Updater, EventConnect)
	return
}

// disconnect 心跳超时,视为断开连接
//
// 由 worker() 扫描收集、在状态迁移执行池里执行(见 migrate.go),不在 daemon 协程上跑:
// 它要抢玩家锁,还要触发业务注册的下线事件(离线结算之类,快慢由业务决定)。
// 开头的 CAS 兼作幂等守卫,重复投递是空操作。
func disconnect(p *player.Player) bool {
	status := atomic.LoadInt32(&p.Status)
	if status != player.StatusConnected {
		return false
	}
	if !atomic.CompareAndSwapInt32(&p.Status, player.StatusConnected, player.StatusDisconnect) {
		return false
	}
	p.KeepAlive(0)
	playersOnline.Add(-1)
	p.Lock()
	defer p.Unlock()
	emitter.Events.Emit(p.Updater, EventDisconnect)
	return true
}

// offline 业务逻辑层面掉线。执行位置与幂等性同 disconnect
func offline(p *player.Player) bool {
	status := atomic.LoadInt32(&p.Status)
	if status != player.StatusDisconnect {
		return false
	}
	if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusOffline) {
		return false
	}
	p.KeepAlive(0)
	p.Lock()
	defer p.Unlock()
	emitter.Events.Emit(p.Updater, EventOffline)
	return true
}

// recycling 进入回收站，StatusNone 直接转为 StatusOffline 不触发事件
func recycling(p *player.Player) {
	status := atomic.LoadInt32(&p.Status)
	if status == player.StatusNone {
		if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusOffline) {
			return
		}
	}
	key := p.Key()
	if _, ok := playersRecycling[key]; !ok {
		playersRecycling[key] = p
		logger.Debug("Players.Recycling uid:%v", p.Uid())
	}
}

// released 释放用户实例,接受 StatusOffline / StatusTerminated
//
// 由 worker() 扫描收集、在状态迁移执行池里执行(见 migrate.go),或由 shutdown 并行调用。
// 🔴 它是整个 daemon 里唯一会做数据库 IO 的动作:Destroy 内含一次 BulkWrite 落库,
// 而且是在玩家锁内做的 —— 这正是它不能留在 daemon 协程里串行跑的原因。
//
// **先 CAS 翻状态,再抢玩家锁**(顺序不能调换):翻早一点能让新来的 Get 立刻按拒绝态返回,
// 不必排队等一把注定用不上的锁;而 Destroy 置空 Updater 仍在锁内,不会与正在 handle 里
// 跑的业务撞上。CAS 同时兼作幂等守卫:重复投递、或与 shutdown 撞车,只有一方会成功。
func released(p *player.Player) (ok bool) {
	status := atomic.LoadInt32(&p.Status)
	if status != player.StatusOffline && status != player.StatusTerminated {
		return false
	}
	if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusReleased) {
		return false
	}

	//加锁等在途业务调用结束。不能用 defer:成功路径要在解锁之后才能 manage.Delete ——
	//Delete 要取管理器写锁,持着玩家锁去抢它会与"持管理器读锁等玩家锁"的一方成环
	p.Lock()
	p.Reset()
	if err := p.Destroy(); err != nil {
		atomic.StoreInt32(&p.Status, status) //还原成进来时的状态,Terminated 不能退化成 Offline 被复活
		p.Unlock()
		logger.Alert("Players.release uid:%v,err:%v", p.Uid(), err)
		return false
	}
	p.Unlock()
	manage.Delete(p.Key())
	return true
}

// submit 把一批玩家的状态迁移投给执行池,返回**没能投出去**的数量
//
// 三档迁移(disconnect / offline / released)的形状完全一样:投递 → 成功则记一行 Debug →
// 失败则计数,所以收成一个函数,worker 里就是三行。
//
// 各迁移函数内部都用 CAS 二次校验状态:快照与执行之间状态变了、或同一个人被下个 tick
// 重复投递,都会安全跳过 —— 这是"投出去就不管了"能成立的前提。
func submit(dict []*player.Player, name string, migrate func(*player.Player) bool) (dropped int) {
	for _, p := range dict {
		if !migratePost(func() {
			if migrate(p) {
				logger.Debug("Players.%v uid:%v", name, p.Uid())
			}
		}) {
			dropped++
		}
	}
	return
}

func worker() {
	defer func() {
		if e := recover(); e != nil {
			logger.Debug("Players worker error:%v \n %v", e, string(debug.Stack()))
		}
	}()
	//dropped 统计本轮没能投进执行池的迁移任务。丢掉不丢状态(下个 tick 会重新收集到),
	//但持续为正说明池子长期满 —— 要么数据库慢,要么 migrateWorker 该调大了,必须看得见。
	var dropped int
	defer func() {
		if dropped > 0 {
			//Trace 在生产环境通常关着,而这正是"该调大 MigrateWorker / 查数据库"的信号,必须看得见
			logger.Alert("Players 状态迁移队列已满,%d 个任务本轮未投递,下个 tick 重试", dropped)
		}
	}()
	if playersRecycling == nil {
		playersRecycling = map[string]*player.Player{}
	}
	now := time.Now().Unix()
	connectedTime := now - Options.ConnectedTime
	disconnectTime := now - Options.DisconnectTime
	offlineTime := now - Options.OfflineTime

	//扫描阶段只做状态判定并收集,不在 Range 内做状态迁移:
	//Range 持有 Manage 读锁,而 disconnect/offline 要抢玩家锁并触发业务事件(可能再抢其他全局锁),
	//持全局读锁等细粒度锁会把登录路径(Manage 写锁)整体挡住,批量掉线时形成全局停顿
	var tot int32
	var down, off, recy, terminate []*player.Player
	manage.Range(func(_ string, p *player.Player) bool {
		tot += 1
		switch atomic.LoadInt32(&p.Status) {
		case player.StatusNone, player.StatusOffline:
			if p.Heartbeat() < offlineTime {
				recy = append(recy, p)
			}
		case player.StatusConnected:
			if p.Heartbeat() <= connectedTime {
				down = append(down, p)
			}
		case player.StatusDisconnect:
			if p.Heartbeat() < disconnectTime {
				off = append(off, p)
			}
		case player.StatusTerminated:
			terminate = append(terminate, p) //踢下线不等超时
		default:
		}
		return true
	})
	playersMemory.Store(tot)

	//以下均已脱离 Manage 读锁。🔴 **迁移动作一律投给执行池**(见 migrate.go):它们都要抢玩家锁,
	//而 released 还会在锁内做一次 BulkWrite —— 留在 daemon 里串行跑的话,扫描周期就由最慢的
	//那个玩家决定。daemon 到这里只剩"谁该迁移到哪个状态"这一件事。
	dropped += submit(down, "Disconnect", disconnect)
	dropped += submit(off, "Offline", offline)
	dropped += submit(terminate, "Terminated", released)

	//recycling 例外:它只往 playersRecycling 这张 daemon 私有的表里记一笔,不抢任何玩家锁、
	//也不碰 Updater。投出去反而要给那张表加锁,留在本协程里最省事。
	for _, p := range recy {
		recycling(p)
	}
	//回收站清理:只有内存压力超过阈值才动手,平时离线玩家就留在缓存里等重连
	ct := tot
	if len(playersRecycling) == 0 || tot < Options.MemoryPlayer+Options.MemoryRelease {
		return
	}
	var dict []*player.Player
	for _, p := range playersRecycling {
		dict = append(dict, p)
	}
	sort.Slice(dict, func(i, j int) bool {
		return dict[i].Heartbeat() < dict[j].Heartbeat()
	})

	//一个 tick 释放一批(ReleaseBatch 个)。释放已经不在 daemon 协程里跑了,分批的理由
	//换成了另外两条:一次退潮把上千个玩家的落库同时压给数据库不是好主意,执行池的队列也吃不下。
	//这一批没轮到的留在回收站,下个 tick 继续。
	var freed int32
	next := map[string]*player.Player{}
	for _, p := range dict {
		if ct > Options.MemoryPlayer && freed < Options.ReleaseBatch {
			//🔴 投出去就从回收站摘掉,**不等结果**:释放失败(Destroy 报错)时 released 会把状态
			//还原成 Offline,下一个 tick 的扫描会把他重新收进回收站 —— 自愈,不必在这里等。
			if migratePost(func() { _ = released(p) }) {
				ct--
				freed++
				continue
			}
			dropped++
		}
		if atomic.LoadInt32(&p.Status) == player.StatusOffline {
			next[p.Key()] = p
		}
	}
	playersRecycling = next
	if freed >= Options.ReleaseBatch && ct > Options.MemoryPlayer {
		logger.Trace("Players.release 本批已放满 %d 个,剩余 %d 待清理", freed, ct-Options.MemoryPlayer)
	}
}

func daemon(ctx context.Context) {
	migrateStart() //状态迁移执行池,daemon 自己只扫描收集
	t := time.Second * time.Duration(Options.Heartbeat)
	timer := time.NewTimer(t)
	defer timer.Stop()
	defer shutdown()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			//扣除 worker 自身耗时,让扫描周期稳定在 t,而不是 t+worker 耗时;
			//worker 超时也不 back-to-back 连跑,留一个调度间隙
			start := time.Now()
			worker()
			if d := t - time.Since(start); d > 0 {
				timer.Reset(d)
			} else {
				timer.Reset(time.Millisecond)
			}
		}
	}
}

func shutdown() {
	//翻状态兼作幂等守卫:停服后 Serviceable 即刻拒绝一切请求,重复调用直接返回
	if !playersState.CompareAndSwap(stateRunning, stateStopped) {
		return
	}
	logger.Alert("收到退出信号，正在保存所有玩家数据")
	start := time.Now()

	//先等执行池排空退出:那批在途的迁移正持着玩家锁在补下线事件、在落库(见 migrateWait);
	//再自己兜一次底 —— ctx 触发时 daemon 可能正好在投递,而池已经退出了(见 migrateDrain)。
	migrateWait()
	migrateDrain()

	//🔴 Range 内**只做快照**,一个玩家锁都不碰。
	//
	//Range 持有 Manage 读锁,而 disconnect/offline 要抢玩家锁并触发业务事件,released 还要
	//回头 manage.Delete(写锁)。在 Range 里做迁移就会成环:这里持读锁等某个玩家锁,
	//而那把玩家锁的持有者(在途请求,或执行池里还没跑完的 released)正在等写锁 ——
	//"服务器都在关了哪还有锁"并不成立,scc 的 ctx Done 不会让在途请求立刻结束。
	//worker() 的扫描阶段(见上面 175 行那段注释)就是为同一个理由把迁移全挪到 Range 外的。
	var rel []*player.Player
	manage.Range(func(_ string, p *player.Player) bool {
		rel = append(rel, p)
		return true
	})

	//离开 Manage 读锁之后再补下线事件,把玩家推到可释放态(released 只接受 Offline/Terminated)
	for _, p := range rel {
		switch atomic.LoadInt32(&p.Status) {
		case player.StatusConnected:
			disconnect(p)
			offline(p)
		case player.StatusDisconnect:
			offline(p)
		case player.StatusNone:
			//预加载进来、从未上线的玩家:没有下线事件要补,直接推到可释放态。
			//🔴 用 CAS 不用 Store:原先这里是无条件 atomic.Store(Offline),会把
			//Released(执行池刚释放完)和 Terminated(被踢,released 本就接受)一并抹掉 ——
			//前者让已销毁的对象又走一遍释放流程,后者把踢人标记降级成普通离线。
			atomic.CompareAndSwapInt32(&p.Status, player.StatusNone, player.StatusOffline)
		}
	}

	//并行释放并等齐:每个脏玩家一次 BulkWrite,串行跑的话停服时间 = 玩家数 × 一次 DB 往返。
	//不能借用执行池 —— 此刻 scc 的 ctx 已经 Done,池里的 worker 正在退出,见 releaseAll。
	failed := releaseAll(rel)
	//没释放成功的只可能是 StatusLocked(正在 Loading)或已经 Released 过的。
	//必须留痕:停服时"有几个玩家没落库"是事后唯一能查的线索。
	if failed > 0 {
		logger.Alert("玩家数据保存完成:共 %d 个,其中 %d 个未能释放,耗时 %v", len(rel), failed, time.Since(start))
	} else {
		logger.Alert("玩家数据保存完成:共 %d 个,耗时 %v", len(rel), time.Since(start))
	}
}
