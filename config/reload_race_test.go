package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type raceHandle struct{}

func (raceHandle) Handle(c *CS, d any) {
	c.ITypes.Add(1001, 60, 0, "race-item")
	c.ITypes.Add(1002, 20, 0, "race-unit")
	c.Process.Set("race", map[int32]int32{1001: 1})
}

func (raceHandle) Verify(c *CS, d any) []error { return nil }

// TestReloadRace 锁定热更的发布语义：读者与 Reload 并发时不得出现数据竞争
// (-race 下必须通过)，且读者任一时刻读到的都是完整快照(ITypes 与 Process 同世代，
// 不会出现新 ITypes 配旧 Process 的撕裂)。
func TestReloadRace(t *testing.T) {
	Register(raceHandle{})

	dir := t.TempDir()
	file := filepath.Join(dir, "race.json")
	if err := os.WriteFile(file, []byte(`{"x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	data := &struct {
		X int `json:"x"`
	}{}
	if err := Reload(data, file); err != nil {
		t.Fatalf("首次加载失败:%v", err)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				s := Load()
				if s.ITypes.GetIType(1001) != 60 {
					t.Error("快照不完整:ITypes 缺条目")
					return
				}
				if s.Process.Get("race") == nil {
					t.Error("快照不完整:Process 缺条目")
					return
				}
				//兼容转发路径一并压测
				if !Is(1001, 60) || GetName(1002) == "" {
					t.Error("转发方法读取异常")
					return
				}
			}
		}()
	}
	for i := 0; i < 300; i++ {
		if err := Reload(data, file); err != nil {
			t.Fatalf("第%v次热更失败:%v", i, err)
		}
	}
	close(done)
	wg.Wait()
}
