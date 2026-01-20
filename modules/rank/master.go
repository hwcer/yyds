package rank

import (
	"context"
	"time"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/logger"
)

// Master 排行榜管理器
// 注意: 排行榜在程序启动时初始化，不需要使用竞态保护
var Master = master{}

type master map[string]*Bucket

func (this master) Get(name string) *Bucket {
	return this[name]
}

// Register 注册排行榜 程序初始化时调用，注册排行榜的名称，最大数量，初始分数，排序类型，处理函数
func (this master) Register(name string, zMax, zScore int64, zType SortType, handle Handle) {
	if _, ok := this[name]; ok {
		logger.Fatal("重复注册排行榜:%v", name)
	}
	this[name] = NewBucket(name, zMax, zScore, zType, handle)
}

func (this master) start() (err error) {
	for _, bucket := range this {
		if err = bucket.start(); err != nil {
			return
		}
	}
	scc.CGO(this.heartbeat)
	return
}

func (this master) heartbeat(ctx context.Context) {
	v := time.Second * Heartbeat
	t := time.NewTimer(v)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			this.worker()
			t.Reset(v)
		}
	}
}

func (this master) worker() {
	defer func() {
		if e := recover(); e != nil {
			logger.Error(e)
		}
	}()
	for _, bucket := range this {
		bucket.heartbeat()
	}
}
