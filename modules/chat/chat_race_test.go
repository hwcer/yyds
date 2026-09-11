package chat

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestChatConcurrentReadWrite 🔴判据回归:Write(多写协程,槽位写+指针推进)与
// Read(读协程,槽位读)并发。曾有槽位普通写(Message 是接口,双字撕裂 UB),
// -race 下必须干净。
func TestChatConcurrentReadWrite(t *testing.T) {
	c := New(64, nil)
	writers := &sync.WaitGroup{}
	stop := make(chan struct{})
	var bad atomic.Int32

	// 4 个读协程持续拉取,验证读到的消息完整(非 nil 且 id 非零)
	for range 4 {
		go func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, rows := c.Read(0, 100, nil)
				for _, m := range rows {
					if m == nil || m.GetId() == 0 {
						bad.Add(1) //撕裂/半构造消息
					}
				}
			}
		}()
	}

	// 4 个写协程各写 500 条
	const perWriter, writerCount = 500, 4
	for range writerCount {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range perWriter {
				if _, err := c.Write("hello", nil, nil); err != nil {
					t.Errorf("write error:%v", err)
					return
				}
			}
		}()
	}
	writers.Wait()
	close(stop)

	if v := bad.Load(); v != 0 {
		t.Fatalf("读到 %d 条不完整消息", v)
	}
	//写完后:全量可读且 id 连续唯一(1..writerCount*perWriter)
	_, rows := c.Read(0, 100, nil)
	if len(rows) != 64 {
		t.Fatalf("缓冲区容量 64,应读满 64 条,拿到 %d", len(rows))
	}
	seen := map[uint64]bool{}
	for _, m := range rows {
		if seen[m.GetId()] {
			t.Fatalf("消息 id 重复:%d", m.GetId())
		}
		seen[m.GetId()] = true
	}
}
