package rank

import (
	"context"
	"fmt"
	"testing"
)

// 溢出清理是延时的(ZCARD > zMax+OverflowThreshold 才触发，且由 5s 心跳驱动)，
// 期间 REDIS 里真实存在 zMax 之外的成员。对外的查询接口必须一律卡在 zMax 之内，
// 否则玩家会看到"第 5200 名"这种榜容量之外的名次。
func TestQueryClampedToZMax(t *testing.T) {
	setupRedis(t)
	const zMax = 5
	b := NewBucket("zmaxclamp", "zmaxclamp", zMax, ScoreUnlimited, SortTypeAsc, swapHandle{}, WithoutTiebreak())
	b.zStmt.Store(NewStatement(1, 0, 0))
	key := b.RedisRankKey(1)
	client.Del(context.Background(), key)
	t.Cleanup(func() { client.Del(context.Background(), key) })

	//塞 8 个(> zMax，但未达 zMax+OverflowThreshold，清理不会触发)
	for i := 1; i <= 8; i++ {
		if err := b.ZAdd(1, fmt.Sprintf("u%d", i), int64(i)); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := client.ZCard(context.Background(), key).Result(); n != 8 {
		t.Fatalf("前置条件:REDIS 里应有 8 个成员，实际 %d", n)
	}

	// ZCard 对外只报 zMax
	if n, err := b.ZCard(1); err != nil || n != zMax {
		t.Errorf("ZCard 应为 %d，实际 %d(err=%v)", zMax, n, err)
	}

	// 榜内成员正常返回名次
	if p, err := b.ZRank(1, "u5", true); err != nil || p.Rank != 4 {
		t.Errorf("u5 应为第 4 名(0基)，实际 %d(err=%v)", p.Rank, err)
	}

	// 🔴 榜外成员必须视为未上榜，而不是如实返回 5/6/7
	for _, uid := range []string{"u6", "u7", "u8"} {
		p, err := b.ZRank(1, uid, true)
		if err != nil {
			t.Fatalf("ZRank(%s): %v", uid, err)
		}
		if p.Rank != -1 {
			t.Errorf("%s 在 zMax 之外，应返回 -1(未上榜)，实际 %d", uid, p.Rank)
		}
	}

	// ZRange 越界要截断
	rs, err := b.ZRange(1, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != zMax {
		t.Errorf("ZRange(0,100) 应截断到 %d 条，实际 %d 条", zMax, len(rs))
	}
	// 起点已在榜外 -> 空
	if rs, err = b.ZRange(1, zMax, 100); err != nil || len(rs) != 0 {
		t.Errorf("ZRange(%d,100) 起点在榜外，应返回空，实际 %d 条(err=%v)", zMax, len(rs), err)
	}

	// ZPlayer 取榜外名次 -> nil
	if p, err := b.ZPlayer(1, zMax+1); err != nil || p != nil {
		t.Errorf("ZPlayer(%d) 榜外应返回 nil，实际 %+v(err=%v)", zMax+1, p, err)
	}

	// Range 遍历(结算发奖用)不得吐出榜外成员
	var got int
	if err = b.Range(1, func(*Player) error { got++; return nil }); err != nil {
		t.Fatal(err)
	}
	if got != zMax {
		t.Errorf("Range 应只遍历 %d 人，实际 %d 人", zMax, got)
	}
}

// zMax==0(不限人数)时不得做任何截断
func TestNoClampWhenUnlimited(t *testing.T) {
	setupRedis(t)
	b := NewBucket("zmaxfree", "zmaxfree", 0, ScoreUnlimited, SortTypeAsc, swapHandle{}, WithoutTiebreak())
	b.zStmt.Store(NewStatement(1, 0, 0))
	key := b.RedisRankKey(1)
	client.Del(context.Background(), key)
	t.Cleanup(func() { client.Del(context.Background(), key) })

	for i := 1; i <= 8; i++ {
		_ = b.ZAdd(1, fmt.Sprintf("u%d", i), int64(i))
	}
	if p, err := b.ZRank(1, "u8", false); err != nil || p.Rank != 7 {
		t.Errorf("不限人数时 u8 应为第 7 名，实际 %d(err=%v)", p.Rank, err)
	}
	if rs, err := b.ZRange(1, 0, 100); err != nil || len(rs) != 8 {
		t.Errorf("不限人数时应返回全部 8 条，实际 %d 条(err=%v)", len(rs), err)
	}
}
