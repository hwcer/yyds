package rank

import (
	"fmt"
	"time"

	"github.com/hwcer/cosgo/redis"
)

var (
	// eraYear 开服时间戳，用于计算排行榜周期和分数格式化
	eraYear int64
	// doomsday 末日时间戳，用于分数格式化时的默认值，设置为开服时间后100年
	doomsday int64
)

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

	et, err := time.Parse(layout, Options.StartTime)
	if err != nil {
		return fmt.Errorf("parse start time error: %v", err)
	}
	eraYear = et.Unix()
	doomsday = eraYear + 100*365*24*60*60
	// 正确返回Master.start()的错误，确保初始化失败时能够向上层报告
	return Master.start()
}

func GetBucket(name string) *Bucket {
	return Master.Get(name)
}

func Register(name string, zMax, zScore int64, zType SortType, plugs Handle) {
	Master.Register(name, zMax, zScore, zType, plugs)
}
