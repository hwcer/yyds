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

// master key 为 ParseName 转换后的确定字符串
type master map[string]*Bucket

func (this master) Get(name any) *Bucket {
	k, err := ParseName(name)
	if err != nil {
		logger.Debug(err)
		return nil
	}
	return this[k]
}

// Register 注册排行榜 程序初始化时调用，注册排行榜的名称，最大数量，初始分数，排序类型，处理函数
//
// name 仅支持字符串和数字
//
// 只能在 Start 之前调用:启动后 Master 会被心跳协程和业务协程并发读取,再写入将导致进程崩溃
func (this master) Register(name any, zMax, zScore int64, zType SortType, handle Handle) {
	if started.Load() {
		logger.Fatal("排行榜必须在 rank.Start 之前注册:%v", name)
		return
	}
	k, err := ParseName(name)
	if err != nil {
		logger.Fatal(err)
		return
	}
	if _, ok := this[k]; ok {
		logger.Fatal("重复注册排行榜:%v", name)
		return
	}
	this[k] = NewBucket(name, k, zMax, zScore, zType, handle)
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
