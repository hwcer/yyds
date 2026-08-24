package rank

import (
	"context"
	"errors"
	"testing"
)

// truceHandle 可切换的休战判定，用来验证 HandleTruce 回调真的被采纳
type truceHandle struct{ on bool }

func (h *truceHandle) Truce(int64) bool                     { return h.on }
func (h *truceHandle) Cycle(skip int64) int64               { return 1 + skip }
func (h *truceHandle) Expire(int64) (int64, int64)          { return 0, 0 }
func (h *truceHandle) Submit(*Bucket, int64) (int64, error) { return 0, nil }

// plainHandle 不实现 HandleTruce —— 应当永不休战
type plainHandle struct{}

func (plainHandle) Cycle(skip int64) int64               { return 1 + skip }
func (plainHandle) Expire(int64) (int64, int64)          { return 0, 0 }
func (plainHandle) Submit(*Bucket, int64) (int64, error) { return 0, nil }

func newTruceBucket(t *testing.T, h Handle) *Bucket {
	b := NewBucket("trucetest", "trucetest", 0, ScoreUnlimited, SortTypeDesc, h, WithoutTiebreak())
	b.zStmt.Store(NewStatement(1, 0, 0))
	client.Del(context.Background(), b.RedisRankKey(1))
	t.Cleanup(func() { client.Del(context.Background(), b.RedisRankKey(1)) })
	return b
}

// TestTruceOptional 不实现 HandleTruce 的榜永不休战
//
// 这是「可选接口」的意义：绝大多数榜没有休战期，不该被逼着写一个恒 false 的方法。
func TestTruceOptional(t *testing.T) {
	setupRedis(t)
	b := newTruceBucket(t, plainHandle{})
	if b.truce != nil {
		t.Fatal("未实现 HandleTruce 时不该断言出非 nil")
	}
	if !b.Writable(1) {
		t.Error("没有休战判定的榜必须恒可写")
	}
	if err := b.ZAdd(1, "u", 10); err != nil {
		t.Errorf("不该报错: %v", err)
	}
}

// TestTruceBlocksWrites 休战期内【玩家驱动】的两条写入路径显式报错
//
// 🔴 重点是 ZAdd：它此前在休战期【静默返回 nil】——玩家扣了票、发了奖、
// 分数却没写进去，调用方拿到 nil 以为成功，日志里什么都看不到。
// 现已与 ZSwap 对齐，统一返回 ErrTruce。
//
// ⚠ 系统驱动的 ZAdds / ZPreset【不在此列】，见 TestTruceAllowsBuildWrites。
func TestTruceBlocksWrites(t *testing.T) {
	setupRedis(t)
	h := &truceHandle{}
	b := newTruceBucket(t, h)

	// 先在非休战期铺两个人，供 ZSwap 用
	if err := b.ZAdd(1, "a", 100); err != nil {
		t.Fatal(err)
	}
	if err := b.ZAdd(1, "c", 200); err != nil {
		t.Fatal(err)
	}

	h.on = true
	if b.Writable(1) {
		t.Error("休战期 Writable 必须为 false")
	}
	if err := b.ZAdd(1, "b", 10); !errors.Is(err, ErrTruce) {
		t.Errorf("休战期 ZAdd 应返回 ErrTruce，实际 %v", err)
	}
	if p, _ := b.ZRank(1, "b", false); p.Rank >= 0 {
		t.Error("休战期被拒的写入不该真的落进榜里")
	}
	if _, err := b.ZSwap(1, "a", "c", SwapIfTargetAhead); !errors.Is(err, ErrTruce) {
		t.Errorf("休战期 ZSwap 应返回 ErrTruce，实际 %v", err)
	}

	// 解除后恢复可写
	h.on = false
	if !b.Writable(1) {
		t.Error("解除休战后必须恢复可写")
	}
	if err := b.ZAdd(1, "b", 10); err != nil {
		t.Errorf("解除休战后不该报错: %v", err)
	}
}

// TestTruceIsCallbackNotWindow 休战由回调实时决定，与「届末 X 秒」无关
//
// 旧签名 Truce() int64 只能表达一种形状：一届一次、贴着届末。
// 本用例的 Expire 恒返回 (0,0)——按旧实现永远算不出休战窗口，
// 而回调说休战就休战。角斗场要的正是这种：届是【周】，休赛却要【每天】一次。
func TestTruceIsCallbackNotWindow(t *testing.T) {
	setupRedis(t)
	h := &truceHandle{on: true}
	b := newTruceBucket(t, h)
	if b.Writable(1) {
		t.Error("回调说休战就该休战，不依赖 Expire 算窗口")
	}
}

// TestTruceAllowsBuildWrites 🔴 休战期内【系统建榜】必须写得进去
//
// 休战约束的是「改变名次」（玩家写分、席位争夺），不是「建立榜单」。
// 而建榜恰恰发生在休战窗口里：换届冻结 → 结算 → 把新一届填好，是同一段时间里的事。
// 若 ZAdds/ZPreset 也判休战，它们在自己唯一的正经用途上永远返回 ErrTruce。
func TestTruceAllowsBuildWrites(t *testing.T) {
	setupRedis(t)
	h := &truceHandle{on: true}
	b := newTruceBucket(t, h)

	if b.Writable(1) {
		t.Fatal("前置状态错误：应处于休战期")
	}
	// 玩家写分被拦
	if err := b.ZAdd(1, "player", 10); !errors.Is(err, ErrTruce) {
		t.Fatalf("休战期 ZAdd 应被拦，实际 %v", err)
	}
	// 系统建榜放行，且必须真的落进榜里
	n, err := b.ZAdds(1, []Member{{Uid: "s1", Score: 10}, {Uid: "s2", Score: 20}})
	if err != nil {
		t.Fatalf("休战期 ZAdds 应放行，实际 %v", err)
	}
	if n != 2 {
		t.Fatalf("应写入 2 条，实际 %v", n)
	}
	if p, _ := b.ZRank(1, "s1", true); p == nil || p.Rank < 0 || p.Score != 10 {
		t.Fatalf("s1 应真的在榜，实际 %+v", p)
	}
	if c, _ := b.ZCard(1); c != 2 {
		t.Fatalf("榜内应为 2 人（玩家那条被拦下），实际 %v", c)
	}
}
