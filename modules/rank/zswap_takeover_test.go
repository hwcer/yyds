package rank

import (
	"context"
	"errors"
	"testing"
)

// 顶替路径:a 在榜外,接手 b 的名次,b 出榜,榜内总数不变
func TestZSwapTakeoverSuccess(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	if err := b.ZAdd(1, "def", 4996); err != nil {
		t.Fatal(err)
	}
	if err := b.ZAdd(1, "other", 4997); err != nil {
		t.Fatal(err)
	}
	r, err := b.ZSwap(1, "atk", "def", SwapAlways)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != SwapKindTakeover {
		t.Fatalf("a 在榜外应为 SwapKindTakeover, 实际 %d", r.Kind)
	}
	if r.ScoreB != 4996 {
		t.Fatalf("接手到的名次应为 4996,实际 %v", r.ScoreB)
	}
	//atk 坐上 4996,def 出榜,总数仍是 2
	if p, e := b.ZRank(1, "atk", true); e != nil || p == nil || p.Score != 4996 {
		t.Fatalf("atk 应在榜且分数 4996:p=%+v err=%v", p, e)
	}
	//ZRank 对不在榜的成员返回 Rank:-1(而非 nil),err 为 nil
	if p, e := b.ZRank(1, "def", true); e != nil || p == nil || p.Rank >= 0 {
		t.Fatalf("def 应已出榜(Rank=-1),实际 %+v err=%v", p, e)
	}
	if n, e := b.ZCard(1); e != nil || n != 2 {
		t.Fatalf("榜内总数应恒为 2,实际 %v err=%v", n, e)
	}
}

// 🔴 语义反转的一条:a 已在榜【不再是错误】,自动走互换
//
// 拆成 ZSwap/ZTakeover 两个 API 时这里返回 ErrTakeoverMemberExists,要求业务改调 ZSwap。
// 但"a 在不在榜"业务查不准(查完到调用落地之间会变),那个错误码逼着每个调用方写重试样板。
// 现在由脚本自己查、自己分派,两种结局都是成功,榜内总数在两条路径下都不变。
func TestZSwapAutoDispatchWhenAttackerOnList(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	if err := b.ZAdd(1, "atk", 100); err != nil {
		t.Fatal(err)
	}
	if err := b.ZAdd(1, "def", 30); err != nil {
		t.Fatal(err)
	}
	r, err := b.ZSwap(1, "atk", "def", SwapAlways)
	if err != nil {
		t.Fatalf("a 在榜应自动走互换而不是报错, 实际 %v", err)
	}
	if r.Kind != SwapKindSwap {
		t.Fatalf("应为 SwapKindSwap, 实际 %d", r.Kind)
	}
	if r.ScoreA != 100 || r.ScoreB != 30 {
		t.Fatalf("返回原分数应为 (100,30), 实际 (%d,%d)", r.ScoreA, r.ScoreB)
	}
	//互换:双方都还在榜,总数不变(顶替时是 a 进 b 出,总数同样不变)
	if p, _ := b.ZRank(1, "atk", true); p.Score != 30 {
		t.Fatalf("atk 应换到 30, 实际 %d", p.Score)
	}
	if p, _ := b.ZRank(1, "def", true); p.Score != 100 {
		t.Fatalf("def 应换到 100, 实际 %d", p.Score)
	}
	if n, _ := b.ZCard(1); n != 2 {
		t.Fatalf("榜内总数不该变,实际 %v", n)
	}
}

// 🔴 顶替路径下 SwapIfTargetAhead 必须恒放行,且不能因 ZRANK 取到 nil 而崩
//
// 这一条同时守着两件事:
//  1. 语义——榜外不在分数空间里,既没分数也没名次,一律视为"排在所有人之后",
//     故 b 恒更靠前、条件恒成立。升序降序都得成立(它只跟"有没有席位"有关)。
//  2. 实现——cond 判定必须留在互换分支【里面】。若被提到分支之前统一做,
//     ZRANK 对榜外的 a 返回 nil(Lua 里是 false),`rb >= ra` 拿 number 跟 boolean 比
//     会直接抛 Lua 运行时错误,而不是返回 ErrSwapCondition。
func TestZSwapTakeoverIgnoresCond(t *testing.T) {
	for _, st := range []SortType{SortTypeAsc, SortTypeDesc} {
		func() {
			setupRedis(t)
			b := newSwapBucket(t, st)
			if err := b.ZAdd(1, "def", 50); err != nil {
				t.Fatal(err)
			}
			r, err := b.ZSwap(1, "atk", "def", SwapIfTargetAhead)
			if err != nil {
				t.Fatalf("SortType=%d 顶替路径应放行, 实际 %v", st, err)
			}
			if r.Kind != SwapKindTakeover || r.ScoreB != 50 {
				t.Fatalf("SortType=%d 结果不对: %+v", st, r)
			}
		}()
	}
}

// 被夺席位方不在榜:换届/退榜后再来,应报 missing 而不是把 atk 凭空塞进榜
func TestZSwapTakeoverRejectsMissingTarget(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	if _, err := b.ZSwap(1, "atk", "gone", SwapAlways); !errors.Is(err, ErrSwapMemberMissing) {
		t.Fatalf("应返回 ErrSwapMemberMissing,实际 %v", err)
	}
	if n, _ := b.ZCard(1); n != 0 {
		t.Fatalf("失败时不该有任何写入,实际 %v", n)
	}
}

// 休战期只读:顶替路径同样要被拦下
func TestZSwapTakeoverTruce(t *testing.T) {
	setupRedis(t)
	b := NewBucket("ztakeovertruce", "ztakeovertruce", 0, ScoreUnlimited, SortTypeAsc, &truceHandle{on: true}, WithoutTiebreak())
	b.zStmt.Store(NewStatement(1, 0, 0))
	client.Del(context.Background(), b.RedisRankKey(1))
	t.Cleanup(func() { client.Del(context.Background(), b.RedisRankKey(1)) })
	if _, err := b.ZSwap(1, "atk", "def", SwapAlways); !errors.Is(err, ErrTruce) {
		t.Fatalf("休战期应返回 ErrTruce,实际 %v", err)
	}
}
