package rank

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func newAddsBucket(t *testing.T, zMax int64, zScore int64, st SortType, opts ...Option) *Bucket {
	b := NewBucket("zaddstest", "zaddstest", zMax, zScore, st, swapHandle{}, opts...)
	b.zStmt.Store(NewStatement(1, 0, 0))
	client.Del(context.Background(), b.RedisRankKey(1))
	t.Cleanup(func() { client.Del(context.Background(), b.RedisRankKey(1)) })
	return b
}

func seatMembers(n int) []Member {
	r := make([]Member, 0, n)
	for i := 1; i <= n; i++ {
		r = append(r, Member{Uid: fmt.Sprintf("x%04d", i), Score: int64(i)})
	}
	return r
}

// TestZAddsWritesAll 批量写入的每一条都要真的落进榜里,分数与逐条 ZAdd 一致
//
// 分片是本接口最容易出错的地方:少 flush 一次尾巴、或分片边界算错,
// 表现是"榜看着建好了、就是少几个人",而少的正是某个分片的边界——
// 之后"取我上方/下方的对手"会在那几个名次上取空。
func TestZAddsWritesAll(t *testing.T) {
	setupRedis(t)
	b := newAddsBucket(t, 0, ScoreUnlimited, SortTypeAsc, WithoutTiebreak())

	const total = zAddsChunk*2 + 137 // 跨两个整分片再多一截,专门压尾巴
	n, err := b.ZAdds(1, seatMembers(total))
	if err != nil {
		t.Fatalf("ZAdds: %v", err)
	}
	if n != total {
		t.Fatalf("应写入 %d 条,实际 %d", total, n)
	}
	if got, _ := b.ZCard(1); got != total {
		t.Fatalf("榜内应有 %d 人,实际 %d", total, got)
	}
	// 首、分片边界、尾各抽一个:score 必须等于原值
	for _, i := range []int{1, zAddsChunk, zAddsChunk + 1, total} {
		uid := fmt.Sprintf("x%04d", i)
		p, err := b.ZRank(1, uid, true)
		if err != nil {
			t.Fatal(err)
		}
		if p.Score != int64(i) {
			t.Errorf("%s 的分数应为 %d,实际 %d", uid, i, p.Score)
		}
	}
}

// TestZAddsSameAsZAdd 批量与逐条必须写出【逐字节相同】的分数
//
// 两条写入路径一旦格式分叉,同一张榜里会并存两种分数(带 tiebreak 小数位的和纯整数的),
// 排序当场错乱。这里对启用了 tiebreak 的榜做对照——它才是会编码小数位的那种。
func TestZAddsSameAsZAdd(t *testing.T) {
	setupRedis(t)
	b := newAddsBucket(t, 0, ScoreUnlimited, SortTypeDesc) // 不加 WithoutTiebreak,走默认编码
	key := b.RedisRankKey(1)

	if err := b.ZAdd(1, "single", 4242); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ZAdds(1, []Member{{Uid: "batch", Score: 4242}}); err != nil {
		t.Fatal(err)
	}
	a, err := client.ZScore(context.Background(), key, "single").Result()
	if err != nil {
		t.Fatal(err)
	}
	c, err := client.ZScore(context.Background(), key, "batch").Result()
	if err != nil {
		t.Fatal(err)
	}
	if a != c {
		t.Errorf("批量与逐条写出的原始分数必须一致: 逐条=%v 批量=%v", a, c)
	}
}

// TestZAddsFilters 入围分数不达标的不写,空 uid 跳过
func TestZAddsFilters(t *testing.T) {
	setupRedis(t)
	// 降序榜、入围分 100:低于 100 的不该进榜
	b := newAddsBucket(t, 0, 100, SortTypeDesc, WithoutTiebreak())
	n, err := b.ZAdds(1, []Member{
		{Uid: "ok1", Score: 100},
		{Uid: "ok2", Score: 500},
		{Uid: "low", Score: 99},
		{Uid: "", Score: 999},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("只有 2 条够格,实际写入 %d", n)
	}
	if p, _ := b.ZRank(1, "low", false); p.Rank >= 0 {
		t.Error("低于入围分的成员不该进榜")
	}
}

// TestZAddsRejectsExpiredCycle 届号不对时【显式报错】,不能静默不写
//
// 这是本接口与单条 ZAdd 有意不同的一点:批量写的调用方多半在建榜,
// 静默不写会留下一张空榜、调用方却以为填好了。
func TestZAddsRejectsExpiredCycle(t *testing.T) {
	setupRedis(t)
	b := newAddsBucket(t, 0, ScoreUnlimited, SortTypeAsc, WithoutTiebreak())
	n, err := b.ZAdds(999, seatMembers(3)) // 999 不是当前届
	if !errors.Is(err, ErrCycleExpired) {
		t.Errorf("应返回 ErrCycleExpired,实际 err=%v", err)
	}
	if n != 0 {
		t.Errorf("拒绝执行时不该写入任何一条,实际 %d", n)
	}
}

// TestZAddsEmpty 空输入是合法的 no-op,不报错
func TestZAddsEmpty(t *testing.T) {
	setupRedis(t)
	b := newAddsBucket(t, 0, ScoreUnlimited, SortTypeAsc, WithoutTiebreak())
	if n, err := b.ZAdds(1, nil); err != nil || n != 0 {
		t.Errorf("空输入应为 no-op, 实际 n=%d err=%v", n, err)
	}
}

// TestZAddsFasterThanLoop 批量相对逐条的实际收益(只记录,不做硬断言)
//
// 不断言倍数:本地 Redis 与跨网段 Redis 的 RTT 差两个数量级,写死阈值必然在某处误报。
func TestZAddsFasterThanLoop(t *testing.T) {
	setupRedis(t)
	const n = 2000
	ms := seatMembers(n)

	b1 := newAddsBucket(t, 0, ScoreUnlimited, SortTypeAsc, WithoutTiebreak())
	t0 := time.Now()
	for i := range ms {
		if err := b1.ZAdd(1, ms[i].Uid, ms[i].Score); err != nil {
			t.Fatal(err)
		}
	}
	loop := time.Since(t0)
	client.Del(context.Background(), b1.RedisRankKey(1))

	b2 := newAddsBucket(t, 0, ScoreUnlimited, SortTypeAsc, WithoutTiebreak())
	t0 = time.Now()
	if _, err := b2.ZAdds(1, ms); err != nil {
		t.Fatal(err)
	}
	batch := time.Since(t0)

	t.Logf("%d 条: 逐条 %v / 批量 %v (分片 %d)", n, loop, batch, zAddsChunk)
	if batch > loop {
		t.Errorf("批量竟然比逐条还慢: %v > %v", batch, loop)
	}
}
