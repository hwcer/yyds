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

// Disconnect 下线,心跳超时,断开连接等
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

// Offline 业务逻辑层面掉线
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
// 先 CAS 翻状态再抢玩家锁:翻早一点能让新来的 Get 立刻按拒绝态返回而不必排队等锁,
// 而 Destroy 置空 Updater 仍然在锁内,不会与正在 handle 里跑的业务协程撞上
func released(p *player.Player) (ok bool) {
	status := atomic.LoadInt32(&p.Status)
	if status != player.StatusOffline && status != player.StatusTerminated {
		return false
	}
	//加锁等待在途业务调用结束,拿到锁后再 CAS,使状态翻转与 Get/Load 的状态检查互斥
	if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusReleased) {
		return false
	}

	//不能用 defer:成功路径要在解锁之后才能 Delete/Close(Close 关 Syncer 必须在锁外)
	p.Lock()
	p.Reset()
	if err := p.Destroy(); err != nil {
		atomic.StoreInt32(&p.Status, status) //还原成进来时的状态,Terminated 不能退化成 Offline 被复活
		p.Unlock()
		logger.Alert("Players.release uid:%v,err:%v", p.Uid(), err)
		return false
	}
	p.Unlock()
	//先摘出管理器再关通道,避免关闭后仍被 Load 到
	ps.Delete(p.Key())
	p.Close()
	return true
}

func worker() {
	defer func() {
		if e := recover(); e != nil {
			logger.Debug("Players worker error:%v \n %v", e, string(debug.Stack()))
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
	ps.Range(func(_ string, p *player.Player) bool {
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

	//以下均已脱离 Manage 读锁;各函数内部都用 CAS 二次校验状态,快照期间状态变了会被安全跳过
	for _, p := range down {
		if disconnect(p) {
			logger.Debug("Players.Disconnect uid:%v", p.Uid())
		}
	}
	for _, p := range off {
		if offline(p) {
			logger.Debug("Players.Offline uid:%v", p.Uid())
		}
	}
	for _, p := range recy {
		recycling(p)
	}
	for _, p := range terminate {
		if released(p) {
			logger.Debug("Players.Terminated uid:%v", p.Uid())
		}
	}
	ct := tot
	recyclingCount := len(playersRecycling)
	if recyclingCount == 0 || tot < Options.MemoryPlayer+Options.MemoryRelease {
		return
	}
	var dict []*player.Player
	for _, p := range playersRecycling {
		dict = append(dict, p)
	}
	sort.Slice(dict, func(i, j int) bool {
		return dict[i].Heartbeat() < dict[j].Heartbeat()
	})

	//单次 tick 最多释放 MemoryRelease 个:释放要拿玩家锁,脏玩家还会产生一次 BulkWrite 往返,
	//不限量的话大批量退潮会把 daemon 协程阻塞到秒级,拖慢整个状态机的判定精度。
	//超出上限的玩家自然落进下面的 else if 留在回收站,下个 tick 继续
	var freed int32
	next := map[string]*player.Player{}
	for _, p := range dict {
		if ct > Options.MemoryPlayer && freed < Options.MemoryRelease && released(p) {
			ct--
			freed++
		} else if atomic.LoadInt32(&p.Status) == player.StatusOffline {
			next[p.Key()] = p
		}
	}
	playersRecycling = next
	if freed >= Options.MemoryRelease && ct > Options.MemoryPlayer {
		logger.Trace("Players.release 达到单次上限 %d,剩余 %d 待清理", freed, ct-Options.MemoryPlayer)
	}
}

func daemon(ctx context.Context) {
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
	//关闭所有用户
	var rel []*player.Player
	ps.Range(func(uid string, p *player.Player) bool {
		switch atomic.LoadInt32(&p.Status) {
		case player.StatusConnected:
			disconnect(p)
			offline(p)
		case player.StatusDisconnect:
			offline(p)
		case player.StatusOffline:
		default:
		}
		atomic.StoreInt32(&p.Status, player.StatusOffline)
		rel = append(rel, p)
		return true
	})
	//释放所有用户,必须在Range外部循环，否则会死锁
	for _, p := range rel {
		_ = released(p)
	}
}
