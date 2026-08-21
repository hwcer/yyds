package rank

import (
	"context"
	"errors"
	"os"
	"testing"

	cosredis "github.com/hwcer/cosgo/redis"
)

// testRedisAddr 集成测试用的 Redis 地址
//
// 默认本地;用环境变量 RANK_TEST_REDIS 覆盖，便于在只有远端 Redis 的机器上跑
// (地址不写死在代码里——每家的开发环境不一样，硬编码谁的地址都不对)。
// 一律用高位 db，别和业务库混在一起。
func testRedisAddr() string {
	if v := os.Getenv("RANK_TEST_REDIS"); v != "" {
		return v
	}
	return "127.0.0.1:6379?password=123456&db=15"
}

// 需要 Redis(默认本地 127.0.0.1:6379)，不可用则跳过
func setupRedis(t *testing.T) {
	t.Helper()
	c, err := cosredis.New(testRedisAddr())
	if err != nil {
		t.Skipf("跳过:本地 Redis 不可用 %v", err)
	}
	if err = c.Ping(context.Background()).Err(); err != nil {
		t.Skipf("跳过:Redis Ping 失败 %v", err)
	}
	client = c
}

type swapHandle struct{}

func (swapHandle) Cycle(skip int64) int64               { return 1 + skip }
func (swapHandle) Expire(int64) (int64, int64)          { return 0, 0 }
func (swapHandle) Submit(*Bucket, int64) (int64, error) { return 0, nil }

func newSwapBucket(t *testing.T, st SortType) *Bucket {
	b := NewBucket("zswaptest", "zswaptest", 0, ScoreUnlimited, st, swapHandle{}, WithoutTiebreak())
	b.zStmt.Store(NewStatement(1, 0, 0))
	client.Del(context.Background(), b.RedisRankKey(1))
	t.Cleanup(func() { client.Del(context.Background(), b.RedisRankKey(1)) })
	return b
}

// 升序榜(分数即名次):挑战名次更靠前者应成功对调
func TestZSwapAscSuccess(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	if err := b.ZAdd(1, "atk", 100); err != nil {
		t.Fatal(err)
	}
	if err := b.ZAdd(1, "def", 30); err != nil {
		t.Fatal(err)
	}
	from, to, err := b.ZSwap(1, "atk", "def", SwapIfTargetAhead)
	if err != nil {
		t.Fatalf("应交换成功: %v", err)
	}
	if from != 100 || to != 30 {
		t.Errorf("返回原分数应为 (100,30), 实际 (%d,%d)", from, to)
	}
	// 交换后 atk 占 30、def 占 100
	if p, _ := b.ZRank(1, "atk", true); p.Score != 30 {
		t.Errorf("atk 交换后应为 30, 实际 %d", p.Score)
	}
	if p, _ := b.ZRank(1, "def", true); p.Score != 100 {
		t.Errorf("def 交换后应为 100, 实际 %d", p.Score)
	}
}

// 目标名次不比自己靠前 -> 拒绝
func TestZSwapAscConditionRejected(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	_ = b.ZAdd(1, "atk", 30)
	_ = b.ZAdd(1, "def", 100) //def 名次更靠后
	_, _, err := b.ZSwap(1, "atk", "def", SwapIfTargetAhead)
	if !errors.Is(err, ErrSwapCondition) {
		t.Fatalf("应返回 ErrSwapCondition, 实际 %v", err)
	}
	if p, _ := b.ZRank(1, "atk", true); p.Score != 30 {
		t.Errorf("拒绝后不得改动分数, atk=%d", p.Score)
	}
}

// 🔴 同分必须显式报错:互换相同分数是 no-op,返回成功会让调用方误以为换了
func TestZSwapEqualScoreRejected(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	_ = b.ZAdd(1, "atk", 50)
	_ = b.ZAdd(1, "def", 50)
	_, _, err := b.ZSwap(1, "atk", "def", SwapIfTargetAhead)
	if !errors.Is(err, ErrSwapEqualScore) {
		t.Fatalf("同分应返回 ErrSwapEqualScore, 实际 %v", err)
	}
	// 无条件模式同样要拒绝——换了也不会改变任何人的名次
	if _, _, err = b.ZSwap(1, "atk", "def", SwapAlways); !errors.Is(err, ErrSwapEqualScore) {
		t.Fatalf("SwapAlways 同分也应拒绝, 实际 %v", err)
	}
}

// 降序榜:分数大者靠前，方向必须反过来判
func TestZSwapDescDirection(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeDesc)
	_ = b.ZAdd(1, "atk", 30)
	_ = b.ZAdd(1, "def", 100) //降序下 def 分高=更靠前
	if _, _, err := b.ZSwap(1, "atk", "def", SwapIfTargetAhead); err != nil {
		t.Fatalf("降序榜挑战高分者应成功: %v", err)
	}
	if p, _ := b.ZRank(1, "atk", true); p.Score != 100 {
		t.Errorf("atk 应拿到 100, 实际 %d", p.Score)
	}
	// 反向:此刻 atk=100 已在前、def=30 在后,由 atk 去挑战 def 应被拒
	if _, _, err := b.ZSwap(1, "atk", "def", SwapIfTargetAhead); !errors.Is(err, ErrSwapCondition) {
		t.Fatalf("降序榜挑战靠后者应被拒, 实际 %v", err)
	}
}

// 一方不在榜
func TestZSwapMemberMissing(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	_ = b.ZAdd(1, "atk", 10)
	_, _, err := b.ZSwap(1, "atk", "ghost", SwapIfTargetAhead)
	if !errors.Is(err, ErrSwapMemberMissing) {
		t.Fatalf("应返回 ErrSwapMemberMissing, 实际 %v", err)
	}
}

// TestZSwapDescRankActuallySwapped 降序榜:交换后【真实名次】必须对调，而不只是分数对调
//
// 单看 score 互换不足以证明正确——降序榜的排序方向与升序相反，
// 若脚本里 ZRANK/ZREVRANK 选错，条件判定会整个反过来。
func TestZSwapDescRankActuallySwapped(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeDesc)
	//降序:分数越大名次越靠前 -> top=300(rank0), mid=200(rank1), low=100(rank2)
	_ = b.ZAdd(1, "top", 300)
	_ = b.ZAdd(1, "mid", 200)
	_ = b.ZAdd(1, "low", 100)

	rankOf := func(uid string) int64 {
		p, err := b.ZRank(1, uid, false)
		if err != nil {
			t.Fatalf("ZRank(%s): %v", uid, err)
		}
		return p.Rank
	}
	if rankOf("low") != 2 || rankOf("top") != 0 {
		t.Fatalf("前置状态错误 low=%d top=%d", rankOf("low"), rankOf("top"))
	}

	//low 挑战 top(更靠前) -> 应成功
	from, to, err := b.ZSwap(1, "low", "top", SwapIfTargetAhead)
	if err != nil {
		t.Fatalf("降序榜挑战更靠前者应成功: %v", err)
	}
	if from != 100 || to != 300 {
		t.Errorf("返回原分数应为 (100,300), 实际 (%d,%d)", from, to)
	}
	//名次必须真的互换:low 变第 0、top 落到第 2,中间的 mid 不受影响
	if r := rankOf("low"); r != 0 {
		t.Errorf("low 交换后名次应为 0, 实际 %d", r)
	}
	if r := rankOf("top"); r != 2 {
		t.Errorf("top 交换后名次应为 2, 实际 %d", r)
	}
	if r := rankOf("mid"); r != 1 {
		t.Errorf("mid 不该被波及, 名次应为 1, 实际 %d", r)
	}
}
