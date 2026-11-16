package rank

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// testPut 测试直接放入一个桶(绕过 NewBucket 参数,与 Register 同一条 COW 发布路径)
func testPut(k string, b *Bucket) {
	Master.mutex.Lock()
	defer Master.mutex.Unlock()
	Master.store(k, b)
}

// TestMasterCOWSnapshotImmutable 已发布的快照永不修改:注册新桶发布的是新 map,
// 旧快照(心跳协程可能正在遍历它)里看不到新桶,也不会被写。
func TestMasterCOWSnapshotImmutable(t *testing.T) {
	Register("cowtest-a", 0, ScoreUnlimited, SortTypeDesc, swapHandle{})
	t.Cleanup(func() { Master.remove("cowtest-a") })
	old := Master.snapshot()
	Register("cowtest-b", 0, ScoreUnlimited, SortTypeDesc, swapHandle{})
	t.Cleanup(func() { Master.remove("cowtest-b") })
	if _, ok := old["cowtest-b"]; ok {
		t.Fatal("旧快照不应看到新注册的桶")
	}
	if Get("cowtest-a") == nil || Get("cowtest-b") == nil {
		t.Fatal("新快照应同时含新旧两个桶")
	}
}

// TestMasterConcurrentRegisterAndRead 读路径无锁:并发注册与读取不竞争(配合 -race 跑)。
func TestMasterConcurrentRegisterAndRead(t *testing.T) {
	const n = 50
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { //模拟业务读 + 心跳遍历
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = Get("cowtest-c-0")
					for range Master.snapshot() {
					}
				}
			}
		}()
	}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("cowtest-c-%d", i)
		Register(name, 0, ScoreUnlimited, SortTypeDesc, swapHandle{})
		t.Cleanup(func() { Master.remove(name) })
	}
	close(stop)
	wg.Wait()
	for i := 0; i < n; i++ {
		if Get(fmt.Sprintf("cowtest-c-%d", i)) == nil {
			t.Fatalf("第 %d 个桶应已注册", i)
		}
	}
}

// TestRegisterAfterStart 启动后注册:桶当场 start(建好当前届),立即可读写。
func TestRegisterAfterStart(t *testing.T) {
	setupRedis(t)
	Master.mutex.Lock()
	running := Master.running
	Master.running = true //模拟已 Start(不起心跳协程)
	Master.mutex.Unlock()
	t.Cleanup(func() {
		Master.mutex.Lock()
		Master.running = running
		Master.mutex.Unlock()
	})

	Register("cowtest-late", 0, ScoreUnlimited, SortTypeDesc, swapHandle{}, WithoutTiebreak())
	b := Get("cowtest-late")
	t.Cleanup(func() {
		Master.remove("cowtest-late")
		client.Del(context.Background(), b.RedisRankKey(1))
		client.Del(context.Background(), b.RedisSettlementKey())
	})
	if b == nil {
		t.Fatal("启动后注册的桶应可取到")
	}
	if stmt := b.statement(); stmt == nil || stmt.zCycle != 1 {
		t.Fatalf("启动后注册应当场建好当前届, 实际 %+v", stmt)
	}
	client.Del(context.Background(), b.RedisRankKey(1))
	if err := ZAdd("cowtest-late", 1, "u1", 10); err != nil {
		t.Fatalf("ZAdd: %v", err)
	}
	if p, err := ZRank("cowtest-late", 1, "u1", true); err != nil || p.Score != 10 {
		t.Fatalf("ZRank = %v, %v", p, err)
	}
}
