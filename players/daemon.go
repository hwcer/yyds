package players

import (
	"context"
	"runtime/debug"
	"sort"
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players/emitter"
	"github.com/hwcer/yyds/players/player"
)

// Connected 连线，不包括断线重连等
func Connected(p *player.Player, meta values.Metadata) (err error) {
	status := p.Status
	gateway := uint64(meta.GetInt64(options.ServicePlayerGateway))
	if gateway == 0 {
		return errors.New("gateway is empty")
	}

	defer func() {
		if err == nil {
			p.KeepAlive(0)
			p.Login = p.Unix()
		}
	}()

	if ip := meta.GetString(options.ServiceClientIp); ip != "" {
		p.ClientIp = ip
	}

	p.Gateway = gateway
	if b := binder.GetContentType(meta, binder.ContentTypeModRes); b != nil {
		p.Binder = b
	}
	// 不同端不同协议顶号
	if status == player.StatusConnected {
		if p.Gateway == gateway {
			emitter.Events.Emit(p.Updater, EventReconnect)
			return
		} else {
			emitter.Events.Emit(p.Updater, EventReplace)
			return
		}
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
	status := p.Status
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
	status := p.Status
	if !(status == player.StatusNone || status == player.StatusDisconnect) {
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

// released 释放用户实例
func released(p *player.Player) (ok bool) {
	status := p.Status
	if status != player.StatusOffline {
		return false
	}
	if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusReleased) {
		return false
	}
	p.Reset()
	if err := p.Destroy(); err == nil {
		ok = true
		ps.Delete(p.Uid())
	} else {
		ok = false
		p.Status = status
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
	//playersReleaseTime++
	now := time.Now().Unix()
	connectedTime := now - Options.ConnectedTime
	disconnectTime := now - Options.DisconnectTime
	offlineTime := now - Options.OfflineTime

	var tot int32
	ps.Range(func(uid string, p *player.Player) bool {
		tot += 1
		//检查掉线情况
		//logger.Debug("uid:%v   status:%v   heartbeat:%v ", p.Uid(), p.status, p.heartbeat)
		switch p.Status {
		case player.StatusNone, player.StatusOffline:
			if p.Heartbeat() < offlineTime {
				if _, ok := playersRecycling[uid]; !ok {
					playersRecycling[uid] = p
					logger.Debug("Players.Recycling uid:%v", uid)
				}
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
		default:
		}

		return true
	})
	playersMemory = tot
	//var rm int
	ct := tot
	recycling := len(playersRecycling)
	//defer func() {
	//	//logger.Debug("当前在线人数:%d  缓存数量:%d  回收站人数:%d  本次清理:%d", playersOnline, tot, recycling, tot-ct)
	//}()

	//清理内存
	if recycling == 0 || tot < Options.MemoryPlayer+Options.MemoryRelease {
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
		} else if p.Status == player.StatusOffline || p.Status == player.StatusNone {
			next[p.Uid()] = p
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
	ps.Range(func(uid string, p *player.Player) bool {
		if p.Status == player.StatusConnected {
			disconnect(p)
			offline(p)
		} else if p.Status == player.StatusDisconnect {
			offline(p)
		}
		_ = released(p)
		return true
	})
	return
}
