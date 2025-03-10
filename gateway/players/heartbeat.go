package players

import (
	"context"
	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/cosgo/session"
	"sync/atomic"
	"time"
)

var started int32

var PlayerTimeout int32 = 360 //N个心跳无活动开始清理
var PlayerHeartbeat = 10      //心跳检查间隔秒

func Start() {
	if atomic.CompareAndSwapInt32(&started, 0, 1) {
		scc.CGO(heartbeat)
	}
}

// heartbeat 启动协程定时清理无效用户
func heartbeat(ctx context.Context) {
	heartbeatTime := time.Duration(PlayerHeartbeat) * time.Second
	ticker := time.NewTimer(heartbeatTime)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scc.Try(doHeartbeat)
			ticker.Reset(heartbeatTime)
		}
	}
}

func doHeartbeat(ctx context.Context) {
	var remove []*session.Data
	Range(func(v *session.Data) bool {
		if v.Heartbeat(1) >= PlayerTimeout {
			remove = append(remove, v)
		}
		return true
	})
	for _, v := range remove {
		Delete(v)
	}
}
