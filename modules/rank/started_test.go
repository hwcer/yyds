package rank

import (
	"errors"
	"testing"
)

// TestNotStartedReturnsError 未启动时读写一律返回错误，而不是空指针崩掉进程
//
// 🔴 **Get(name) != nil 不代表已启动**：Register 在各模块的 init 里就把桶填进
// Master 了，而 Start 要等 Redis 配置就绪才调用。此前业务只判 Get 非空就去 ZCard，
// 未配 Redis 的环境下直接在 Redis.ZCard 上空指针——而 panic 被 cosgo 的事件 recover
// 吞掉后，日志里只剩框架帧，完全看不出是谁调的（2026-08-21 真实事故）。
func TestNotStartedReturnsError(t *testing.T) {
	old := Redis
	Redis = nil
	t.Cleanup(func() { Redis = old })

	b := NewBucket("startedtest", "startedtest", 0, ScoreUnlimited, SortTypeDesc, swapHandle{})
	Master["startedtest"] = b
	t.Cleanup(func() { delete(Master, "startedtest") })

	// Get 仍能取到桶——这正是它不能用来判「能不能用」的原因
	if Get("startedtest") == nil {
		t.Fatal("Get 应能取到已注册的桶")
	}
	if Started() {
		t.Fatal("Redis 为 nil 时 Started 必须为 false")
	}

	if _, err := ZCard("startedtest", 1); !errors.Is(err, ErrNotStarted) {
		t.Errorf("ZCard 应返回 ErrNotStarted, 实际 %v", err)
	}
	if err := ZAdd("startedtest", 1, "u", 1); !errors.Is(err, ErrNotStarted) {
		t.Errorf("ZAdd 应返回 ErrNotStarted, 实际 %v", err)
	}
	if _, err := ZAdds("startedtest", 1, []Member{{Uid: "u", Score: 1}}); !errors.Is(err, ErrNotStarted) {
		t.Errorf("ZAdds 应返回 ErrNotStarted, 实际 %v", err)
	}
	if _, err := ZRank("startedtest", 1, "u", false); !errors.Is(err, ErrNotStarted) {
		t.Errorf("ZRank 应返回 ErrNotStarted, 实际 %v", err)
	}
	if _, err := ZRange("startedtest", 1, 0, 1); !errors.Is(err, ErrNotStarted) {
		t.Errorf("ZRange 应返回 ErrNotStarted, 实际 %v", err)
	}
}

// TestCycleWorksWithoutRedis 只算周期的接口不依赖 Redis
//
// Cycle / Expire / Writable 都是纯计算，不碰 Redis。让它们也要求「已启动」
// 会把「未配 Redis 时届号照样算得出来」这个既有行为改掉，业务侧的展示与判断都会受影响。
func TestCycleWorksWithoutRedis(t *testing.T) {
	old := Redis
	Redis = nil
	t.Cleanup(func() { Redis = old })

	b := NewBucket("cycletest", "cycletest", 0, ScoreUnlimited, SortTypeDesc, swapHandle{})
	Master["cycletest"] = b
	t.Cleanup(func() { delete(Master, "cycletest") })

	if c := Cycle("cycletest", 0); c != 1 {
		t.Errorf("未启动时 Cycle 仍应算得出来, 实际 %v", c)
	}
	if !Writable("cycletest", 1) {
		t.Error("未启动时 Writable 仍应可判定")
	}
}
