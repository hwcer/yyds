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

	if ip := meta.GetString(gwcfg.ServiceMetadataClientIp); ip != "" {
		p.ClientIp = ip
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
	atomic.AddInt32(&playersOnline, 1)
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
	atomic.AddInt32(&playersOnline, -1)
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

// released 释放用户实例
func released(p *player.Player) (ok bool) {
	status := atomic.LoadInt32(&p.Status)
	if status != player.StatusOffline {
		return false
	}
	if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusReleased) {
		return false
	}
	p.Reset()
	if err := p.Destroy(); err == nil {
		ok = true
		ps.Delete(p.Key())
	} else {
		ok = false
		atomic.StoreInt32(&p.Status, status)
		logger.Alert("Players.release uid:%v,err:%v", p.Uid(), err)
	}
	return
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

	var tot int32
	ps.Range(func(uid string, p *player.Player) bool {
		tot += 1
		switch atomic.LoadInt32(&p.Status) {
		case player.StatusNone, player.StatusOffline:
			if p.Heartbeat() < offlineTime {
				recycling(p)
			}
		case player.StatusConnected:
			if p.Heartbeat() <= connectedTime {
				disconnect(p)
				logger.Debug("Players.Disconnect uid:%v", uid)
			}
		case player.StatusDisconnect:
			if p.Heartbeat() < disconnectTime {
				offline(p)
				logger.Debug("Players.Offline uid:%v", uid)
			}
		}
		return true
	})
	playersMemory = tot
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

	next := map[string]*player.Player{}
	for _, p := range dict {
		if ct > Options.MemoryPlayer && released(p) {
			ct--
		} else if atomic.LoadInt32(&p.Status) == player.StatusOffline {
			next[p.Key()] = p
		}
	}
	playersRecycling = next
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
			worker()
			timer.Reset(t)
		}
	}
}

func shutdown() {
	if !atomic.CompareAndSwapInt32(&playersStarted, 1, 0) {
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
