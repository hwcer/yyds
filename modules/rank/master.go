package rank

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/logger"
)

// Master 排行榜管理器
//
// # 并发模型:写时复制(COW)
//
// 读(Get / 心跳遍历)远多于写(Register),且读在业务热路径上:读路径**不加锁**,
// 直接原子取当前快照;写在 mutex 下复制一份新 map、加入新桶、再原子发布。
// 已发布的快照永不修改,所以心跳协程遍历旧快照、业务协程读新快照都不会竞争。
//
// 因此 Register 在 Start 之前、之后都能调用:启动后注册的桶当场完成 start(建当前届、
// 检查未结算届),发布后下一轮心跳即被驱动。典型场景:活动排行按配置热更新增榜。
var Master = &master{}

// master key 为 ParseName 转换后的确定字符串
type master struct {
	mutex   sync.Mutex                         //串行化写入(Register)以及与 start 的交接
	buckets atomic.Pointer[map[string]*Bucket] //当前快照,发布后只读
	running bool                               //start 已执行(由 mutex 保护):之后注册的桶由 Register 自己 start
}

// snapshot 当前快照(只读,可能为 nil——读 nil map 合法)
func (this *master) snapshot() map[string]*Bucket {
	if p := this.buckets.Load(); p != nil {
		return *p
	}
	return nil
}

func (this *master) Get(name any) *Bucket {
	k, err := ParseName(name)
	if err != nil {
		logger.Debug(err)
		return nil
	}
	return this.snapshot()[k]
}

// Register 注册排行榜:名称,最大数量,初始分数,排序类型,处理函数
//
// name 仅支持字符串和数字;同名重复注册直接 Fatal(业务侧先用 Get 判断是否已注册)。
//
// Start 之前、之后均可调用(见 Master 的并发模型)。启动后注册时会在持锁期间调用
// 桶的 start(其中会回调 handle.Cycle / handle.Expire):**handle 的这两个回调里不得再调
// Register**,否则自死锁。
func (this *master) Register(name any, zMax, zScore int64, zType SortType, handle Handle, opts ...Option) {
	k, err := ParseName(name)
	if err != nil {
		logger.Fatal(err)
		return
	}
	b := NewBucket(name, k, zMax, zScore, zType, handle, opts...)
	this.mutex.Lock()
	defer this.mutex.Unlock()
	if _, ok := this.snapshot()[k]; ok {
		logger.Fatal("重复注册排行榜:%v", name)
		return
	}
	if this.running {
		//启动后注册:先 start 再发布,保证心跳与业务拿到的桶已建好当前届
		if err = b.start(); err != nil {
			logger.Error("排行榜启动失败:%v err:%v", name, err)
		}
	}
	this.store(k, b)
}

// store 复制当前快照、加入新桶后原子发布(调用方持 mutex)
func (this *master) store(k string, b *Bucket) {
	old := this.snapshot()
	m := make(map[string]*Bucket, len(old)+1)
	for key, v := range old {
		m[key] = v
	}
	m[k] = b
	this.buckets.Store(&m)
}

// remove 复制当前快照、去掉指定桶后原子发布(仅测试清理用)
func (this *master) remove(k string) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	old := this.snapshot()
	m := make(map[string]*Bucket, len(old))
	for key, v := range old {
		if key != k {
			m[key] = v
		}
	}
	this.buckets.Store(&m)
}

func (this *master) start() (err error) {
	//持锁完成「启动已注册的桶 + 置 running」:与并发的 Register 交接,
	//保证每个桶恰好被 start 一次(要么在这里,要么在 Register 里)
	this.mutex.Lock()
	for _, bucket := range this.snapshot() {
		if err = bucket.start(); err != nil {
			this.mutex.Unlock()
			return
		}
	}
	this.running = true
	this.mutex.Unlock()
	scc.CGO(this.heartbeat)
	return
}

func (this *master) heartbeat(ctx context.Context) {
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

func (this *master) worker() {
	defer func() {
		if e := recover(); e != nil {
			logger.Error(e)
		}
	}()
	for _, bucket := range this.snapshot() {
		bucket.heartbeat()
	}
}
