package rank

import (
	"context"
	"errors"
	"testing"
)

// 榜外玩家顶替榜内玩家:接手其名次,原主出榜,榜内总数不变
func TestZTakeoverSuccess(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	if err := b.ZAdd(1, "def", 4996); err != nil {
		t.Fatal(err)
	}
	if err := b.ZAdd(1, "other", 4997); err != nil {
		t.Fatal(err)
	}
	score, err := b.ZTakeover(1, "atk", "def")
	if err != nil {
		t.Fatal(err)
	}
	if score != 4996 {
		t.Fatalf("接手到的名次应为 4996,实际 %v", score)
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

// 顶替方已在榜上:必须拒绝。放行会把 atk 原来那一席凭空丢掉(总数 -1)
func TestZTakeoverRejectsMemberOnList(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	if err := b.ZAdd(1, "atk", 100); err != nil {
		t.Fatal(err)
	}
	if err := b.ZAdd(1, "def", 30); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ZTakeover(1, "atk", "def"); !errors.Is(err, ErrTakeoverMemberExists) {
		t.Fatalf("应返回 ErrTakeoverMemberExists,实际 %v", err)
	}
	if n, _ := b.ZCard(1); n != 2 {
		t.Fatalf("拒绝后榜内总数不该变,实际 %v", n)
	}
}

// 被顶替方不在榜:换届/退榜后再来顶替,应报 missing 而不是把 atk 凭空塞进榜
func TestZTakeoverRejectsMissingTarget(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	if _, err := b.ZTakeover(1, "atk", "gone"); !errors.Is(err, ErrSwapMemberMissing) {
		t.Fatalf("应返回 ErrSwapMemberMissing,实际 %v", err)
	}
	if n, _ := b.ZCard(1); n != 0 {
		t.Fatalf("失败时不该有任何写入,实际 %v", n)
	}
}

// 休战期只读:顶替同样要被拦下
func TestZTakeoverTruce(t *testing.T) {
	setupRedis(t)
	b := NewBucket("ztakeovertruce", "ztakeovertruce", 0, ScoreUnlimited, SortTypeAsc, &truceHandle{on: true}, WithoutTiebreak())
	b.zStmt.Store(NewStatement(1, 0, 0))
	client.Del(context.Background(), b.RedisRankKey(1))
	t.Cleanup(func() { client.Del(context.Background(), b.RedisRankKey(1)) })
	if _, err := b.ZTakeover(1, "atk", "def"); !errors.Is(err, ErrTruce) {
		t.Fatalf("休战期应返回 ErrTruce,实际 %v", err)
	}
}
