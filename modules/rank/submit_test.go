package rank

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

type fakeHandle struct {
	calls   atomic.Int64
	cycles  sync.Map //记录每个期数被结算的次数
	failing atomic.Bool
}

func (h *fakeHandle) Truce() int64                      { return 0 }
func (h *fakeHandle) Cycle(skip int64) int64            { return 1 + skip }
func (h *fakeHandle) Expire(cycle int64) (int64, int64) { return 0, 0 }
func (h *fakeHandle) Submit(b *Bucket, cycle int64) (int64, error) {
	h.calls.Add(1)
	n, _ := h.cycles.LoadOrStore(cycle, new(atomic.Int64))
	n.(*atomic.Int64).Add(1)
	if h.failing.Load() {
		return 0, errors.New("boom")
	}
	return 0, nil
}

func (h *fakeHandle) count(cycle int64) int64 {
	if n, ok := h.cycles.Load(cycle); ok {
		return n.(*atomic.Int64).Load()
	}
	return 0
}

// newTestBucket 构造一个不触碰 Redis 的 Bucket:zMax=0 跳过 mayKeeper
func newTestBucket(h Handle) *Bucket {
	return NewBucket("t", "t", 0, ScoreUnlimited, SortTypeDesc, h)
}

// Submit 只入队,不得自己执行结算
func TestSubmitOnlyEnqueues(t *testing.T) {
	h := &fakeHandle{}
	b := newTestBucket(h)
	if err := b.Submit(7); err != nil {
		t.Fatal(err)
	}
	if h.calls.Load() != 0 {
		t.Fatalf("Submit 不应直接结算,却调用了 handle.Submit %d 次", h.calls.Load())
	}
	if len(b.submits) != 1 || b.submits[0].zCycle != 7 {
		t.Fatalf("入队失败: %+v", b.submits)
	}
}

// 并发 Submit 同一期数,队列中只能有一份,心跳只结算一次
func TestSubmitConcurrentNoDuplicate(t *testing.T) {
	h := &fakeHandle{failing: atomic.Bool{}}
	h.failing.Store(true) //失败以避开需要 Redis 的收尾步骤
	b := newTestBucket(h)

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = b.Submit(7) }()
	}
	wg.Wait()

	if len(b.submits) != 1 {
		t.Fatalf("并发入队产生了%d份,期望1份", len(b.submits))
	}
	b.heartbeat() //心跳消费一次
	if got := h.count(7); got != 1 {
		t.Fatalf("cycle=7 被结算%d次,期望1次", got)
	}
}

// 手动入队后由心跳完成结算
func TestSubmitSettledByHeartbeat(t *testing.T) {
	h := &fakeHandle{}
	h.failing.Store(true)
	b := newTestBucket(h)
	_ = b.Submit(7)
	if h.count(7) != 0 {
		t.Fatal("入队阶段不应结算")
	}
	b.heartbeat()
	if got := h.count(7); got != 1 {
		t.Fatalf("心跳后应结算1次,实际%d次", got)
	}
	if len(b.submits) != 0 {
		t.Fatalf("结算后队列应清空,剩余%d项", len(b.submits))
	}
}

// 每次心跳只处理一个,避免阻塞心跳协程
func TestHeartbeatDrainsOnePerTick(t *testing.T) {
	h := &fakeHandle{}
	h.failing.Store(true)
	b := newTestBucket(h)
	for _, c := range []int64{7, 8, 9} {
		_ = b.Submit(c)
	}
	b.heartbeat()
	if h.calls.Load() != 1 {
		t.Fatalf("单次心跳结算了%d个,期望1个", h.calls.Load())
	}
	b.heartbeat()
	b.heartbeat()
	if h.calls.Load() != 3 {
		t.Fatalf("三次心跳后应结算3个,实际%d个", h.calls.Load())
	}
}

// 不同期数互不影响
func TestSubmitDifferentCycles(t *testing.T) {
	h := &fakeHandle{}
	h.failing.Store(true)
	b := newTestBucket(h)
	_ = b.Submit(7)
	_ = b.Submit(8)
	if len(b.submits) != 2 {
		t.Fatalf("不同期数应各自入队,实际%d项", len(b.submits))
	}
}

// 期数为0必须报错:手动补发要显式指定,不能默认成当前届
func TestSubmitRejectsZeroCycle(t *testing.T) {
	b := newTestBucket(&fakeHandle{})
	if err := b.Submit(0); err == nil {
		t.Fatal("cycle=0 应被拒绝")
	}
	if len(b.submits) != 0 {
		t.Fatal("被拒绝的请求不应入队")
	}
}
