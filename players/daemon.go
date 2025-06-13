package players

import (
	"context"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players/player"
	"runtime/debug"
	"sort"
	"sync/atomic"
	"time"
)

// Connect 连线，不包括断线重连等
func Connect(p *player.Player, meta values.Metadata) (err error) {
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
	p.Gateway = gateway
	if b := binder.GetContentType(meta, binder.ContentTypeModRes); b != nil {
		p.Binder = b
	}
	// 不同端不同协议顶号
	if status == player.StatusConnected {
		if p.Gateway == gateway {
			updater.Emit(p.Updater, player.EventReconnect)
			return
		} else {
			updater.Emit(p.Updater, player.EventReplace)
			return
		}
	} else if status == player.StatusNone || status == player.StatusDisconnect || status == player.StatusRecycling {
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
	updater.Emit(p.Updater, player.EventConnect)
	return
}

// Disconnect 下线,心跳超时,断开连接等
func Disconnect(p *player.Player) bool {
	status := p.Status
	if status != player.StatusConnected {
		return false
	}
	if !atomic.CompareAndSwapInt32(&p.Status, player.StatusConnected, player.StatusDisconnect) {
		return false
	}
	p.KeepAlive(0)
	atomic.AddInt32(&playersOnline, -1)
	updater.Emit(p.Updater, player.EventDisconnect)
	return true
}

// recycling 进入回收站等待回收
func recycling(p *player.Player) bool {
	status := p.Status
	if !(status == player.StatusNone || status == player.StatusDisconnect) {
		return false
	}
	if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusRecycling) {
		return false
	}
	p.KeepAlive(0)
	return true
}

// release 释放用户实例
func release(p *player.Player) (ok bool) {
	status := p.Status
	if status != player.StatusRecycling {
		return false
	}
	if !atomic.CompareAndSwapInt32(&p.Status, status, player.StatusRelease) {
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
	playersReleaseTime++
	now := times.Now().Unix()
	offlineTime := now - PlayersDisconnect
	releaseTime := now - PlayersRelease

	var tot int
	ps.Range(func(uid string, p *player.Player) bool {
		tot += 1
		//检查掉线情况
		//logger.Debug("uid:%v   status:%v   heartbeat:%v ", p.Uid(), p.status, p.heartbeat)

		if p.Status == player.StatusConnected {
			if p.Heartbeat() <= offlineTime {
				Disconnect(p)
			} else if _, ok := playersRecycling[uid]; ok {
				delete(playersRecycling, uid)
			}
		} else if p.Status == player.StatusNone || p.Status == player.StatusDisconnect {
			if p.Heartbeat() < releaseTime && recycling(p) {
				playersRecycling[uid] = p
			}
		} else if p.Status == player.StatusRecycling || p.Status == player.StatusRelease {
			playersRecycling[uid] = p
		}

		return true
	})
	//var rm int
	ct := tot
	if playersReleaseTime >= Options.ReleaseTime {
		defer func() {
			playersReleaseTime = 0
			if n := tot - ct; n > 0 {
				logger.Trace("当前在线人数:%v  缓存数量:%v  本次清理:%v", playersOnline, tot, n)
			}
		}()
	}

	//清理内存
	if !(playersReleaseTime >= Options.ReleaseTime && len(playersRecycling) > 0 && tot > Options.MemoryPlayer+Options.MemoryRelease) {
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
		if ct > Options.MemoryPlayer && release(p) {
			ct--
		}
		if p.Status == player.StatusRecycling {
			next[p.Uid()] = p
		}
	}
	playersRecycling = next
}

func daemon(ctx context.Context) {
	t := time.Second * PlayersHeartbeat
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
	//关闭所有用户
	ps.Range(func(uid string, p *player.Player) bool {
		_ = release(p)
		return true
	})
	return
}
