package rank

import (
	"context"
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
	old := client
	client = nil
	t.Cleanup(func() { client = old })

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
	old := client
	client = nil
	t.Cleanup(func() { client = old })

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

// TestServiceAPIWorksWhenStarted 已启动时包级 API 必须真的能用
//
// 🔴 这条看着废话，但它抓到过一个真 bug：`bucket()` 内部被误写成调用自身，
// 未启动时因为提前 return 而不显形，一旦启动就无限递归 → 栈溢出。
// 框架原有的用例全走 Bucket 方法，**从没走过包级 API 的正常路径**，所以一个都没挂。
func TestServiceAPIWorksWhenStarted(t *testing.T) {
	setupRedis(t)
	b := NewBucket("svcapitest", "svcapitest", 0, ScoreUnlimited, SortTypeDesc, swapHandle{}, WithoutTiebreak())
	b.zStmt.Store(NewStatement(1, 0, 0))
	Master["svcapitest"] = b
	t.Cleanup(func() {
		delete(Master, "svcapitest")
		client.Del(context.Background(), b.RedisRankKey(1))
	})
	client.Del(context.Background(), b.RedisRankKey(1))

	if !Started() {
		t.Fatal("setupRedis 之后应为已启动")
	}
	if err := ZAdd("svcapitest", 1, "u1", 100); err != nil {
		t.Fatalf("ZAdd: %v", err)
	}
	if n, err := ZCard("svcapitest", 1); err != nil || n != 1 {
		t.Fatalf("ZCard = %v, %v; want 1, nil", n, err)
	}
	if p, err := ZRank("svcapitest", 1, "u1", true); err != nil || p.Score != 100 {
		t.Fatalf("ZRank = %v, %v", p, err)
	}
	if _, err := ZAdds("svcapitest", 1, []Member{{Uid: "u2", Score: 50}}); err != nil {
		t.Fatalf("ZAdds: %v", err)
	}
	if rows, err := ZRange("svcapitest", 1, 0, 10); err != nil || len(rows) != 2 {
		t.Fatalf("ZRange = %v 条, %v; want 2", len(rows), err)
	}
}

// TestRedisAccessorIsReadOnly 连接只能由 Start 写入，外部只读
//
// 🔴 它曾经是导出的 `Redis` 变量，于是外部可以直接赋值——而 Start 开头的
// `if client != nil { return nil }` 本意是防重复启动，一旦有人提前赋了值，
// Start 就整个空转：ShareId/ServerId 不设（REDIS key 丢掉分区前缀，多服共用一个
// Redis 时互相踩）、started 不置位、Master.start() 不执行（statement 没建、
// 心跳没起、未结算届没检查）。而且全程不报错。
//
// 封死写入口之后，`client != nil` 才真正等价于「Start 调用过」，Started() 也才可信。
func TestRedisAccessorIsReadOnly(t *testing.T) {
	setupRedis(t)
	if Redis() == nil {
		t.Fatal("已启动时 Redis() 不该为 nil")
	}
	if Redis() != client {
		t.Error("Redis() 应返回内部连接本身")
	}
	// 未启动时两者一致地为 nil
	old := client
	client = nil
	t.Cleanup(func() { client = old })
	if Redis() != nil {
		t.Error("未启动时 Redis() 应为 nil")
	}
	if Started() {
		t.Error("未启动时 Started 应为 false")
	}
}
