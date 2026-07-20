package rank

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/redis"
)

// eraYear 开服时间戳，Statement 未指定开始时间时的兜底值
var eraYear int64

// started Start 之后禁止再注册排行榜
//
// Master 是裸 map,心跳协程会遍历它,业务协程会读它,
// 启动后再写入等于并发读写 map,会直接 fatal 而不是可恢复的竞态
var started atomic.Bool

const layout = "2006-01-02 15:04:05-0700"

func Start(redis *redis.Client, sharId string, serverId int32) (err error) {
	if redis == nil {
		return fmt.Errorf("rank redis is nil")
	}
	if Redis != nil {
		return nil
	}
	Redis = redis
	Options.ShareId = sharId
	Options.ServerId = serverId
	//置位后 Register 不再接受新注册,Master 自此只读
	started.Store(true)

	et, err := time.Parse(layout, Options.StartTime)
	if err != nil {
		return fmt.Errorf("parse start time error: %v", err)
	}
	eraYear = et.Unix()
	// 正确返回Master.start()的错误，确保初始化失败时能够向上层报告
	return Master.start()
}

func GetBucket(name any) *Bucket {
	return Master.Get(name)
}

// Register 注册排行榜,只能在 init 阶段(Start 之前)调用
func Register(name any, zMax, zScore int64, zType SortType, plugs Handle) {
	Master.Register(name, zMax, zScore, zType, plugs)
}
