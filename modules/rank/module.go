package rank

import (
	"fmt"
	"time"

	"github.com/hwcer/cosgo/redis"
)

// eraYear 开服时间戳，Statement 未指定开始时间时的兜底值
var eraYear int64

const layout = "2006-01-02 15:04:05-0700"

func Start(redis *redis.Client, sharId string, serverId int32) (err error) {
	if redis == nil {
		return fmt.Errorf("rank redis is nil")
	}
	if client != nil {
		return nil
	}
	client = redis
	Options.ShareId = sharId
	Options.ServerId = serverId

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

// Register 注册排行榜。Start 之前、之后均可调用(Master 写时复制,读路径无锁):
// 启动后注册的桶当场建好当前届并检查未结算届,下一轮心跳起即被驱动。
// 同名重复注册 Fatal——运行期按配置增榜的业务先用 Get 判断是否已注册。
//
// opts 可选项,如 WithoutTiebreak() 关闭同分tiebreak(令 score 保持纯整数)
func Register(name any, zMax, zScore int64, zType SortType, plugs Handle, opts ...Option) {
	Master.Register(name, zMax, zScore, zType, plugs, opts...)
}
