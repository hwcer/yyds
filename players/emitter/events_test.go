package emitter

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hwcer/cosgo/phase"
	"github.com/hwcer/updater"
)

// TestOnAfterSealed 封板后仍可注册,且新监听立即生效(COW 发布)。
func TestOnAfterSealed(t *testing.T) {
	old := phase.Get()
	phase.Set(phase.Started)
	t.Cleanup(func() { phase.Set(old) })

	const ev int32 = -9001
	var n atomic.Int32
	On(ev, func(_ *updater.Updater, vs ...int32) { n.Add(vs[0]) })
	On(ev, func(_ *updater.Updater, vs ...int32) { n.Add(vs[0]) }, true)
	dispatch(nil, ev, []int32{2}, false)
	dispatch(nil, ev, []int32{3}, true)
	if n.Load() != 5 {
		t.Fatalf("封板后注册的惰性 / 急切监听都应生效, 实际累计 %v", n.Load())
	}
}

// TestOnSnapshotImmutable 已发布的快照不被后续注册改动(派发中的协程读的是旧快照)。
func TestOnSnapshotImmutable(t *testing.T) {
	const ev int32 = -9002
	On(ev, func(*updater.Updater, ...int32) {})
	snap := *events.Load()
	before := len(snap[ev])
	On(ev, func(*updater.Updater, ...int32) {})
	if len(snap[ev]) != before {
		t.Fatal("旧快照里的监听切片不应被追加")
	}
	if len((*events.Load())[ev]) != before+1 {
		t.Fatal("新快照应多一个监听")
	}
}

// TestOnConcurrentDispatch 派发无锁读与并发注册不竞争(配合 -race 跑)。
func TestOnConcurrentDispatch(t *testing.T) {
	const ev int32 = -9003
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					dispatch(nil, ev, []int32{1}, false)
				}
			}
		}()
	}
	for i := 0; i < 100; i++ {
		On(ev, func(*updater.Updater, ...int32) {})
	}
	close(stop)
	wg.Wait()
	if got := len((*events.Load())[ev]); got != 100 {
		t.Fatalf("应注册 100 个监听, 实际 %v", got)
	}
}
