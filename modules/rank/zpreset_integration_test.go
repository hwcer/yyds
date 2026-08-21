package rank

import (
	"context"
	"errors"
	"testing"
)

func presetMembers(n int) []Member {
	ms := make([]Member, 0, n)
	for i := 1; i <= n; i++ {
		ms = append(ms, Member{Uid: uidOf(i), Score: int64(i)})
	}
	return ms
}

func uidOf(i int) string {
	return string(rune('a'+i%26)) + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// 预置未来一届:写进去、总数对、当前届不受影响
func TestZPresetFutureCycle(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc) //zStmt = 第 1 届
	if err := b.ZAdd(1, "live", 10); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Del(context.Background(), b.RedisRankKey(2)) })

	n, err := b.ZPreset(2, presetMembers(50))
	if err != nil {
		t.Fatal(err)
	}
	if n != 50 {
		t.Fatalf("应写入 50 条,实际 %v", n)
	}
	got, err := client.ZCard(context.Background(), b.RedisRankKey(2)).Result()
	if err != nil || got != 50 {
		t.Fatalf("第 2 届应有 50 条,实际 %v err=%v", got, err)
	}
	//当前届纹丝不动
	cur, _ := client.ZCard(context.Background(), b.RedisRankKey(1)).Result()
	if cur != 1 {
		t.Fatalf("当前届不该被动到,实际 %v 条", cur)
	}
	//临时键必须已被 RENAME 掉,不能留垃圾
	if e, _ := client.Exists(context.Background(), b.RedisRankKey(2)+"-preset").Result(); e != 0 {
		t.Fatal("临时键没清掉")
	}
}

// 目标届已有数据必须拒绝,不能覆盖也不能叠加
func TestZPresetRejectsNonEmptyTarget(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	t.Cleanup(func() { client.Del(context.Background(), b.RedisRankKey(2)) })

	if _, err := b.ZPreset(2, presetMembers(50)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ZPreset(2, presetMembers(20)); !errors.Is(err, ErrPresetNotEmpty) {
		t.Fatalf("第二次预置应返回 ErrPresetNotEmpty,实际 %v", err)
	}
	got, _ := client.ZCard(context.Background(), b.RedisRankKey(2)).Result()
	if got != 50 {
		t.Fatalf("被拒绝时原数据必须原样保留,应为 50 条,实际 %v", got)
	}
	//临时键不能留下来
	if e, _ := client.Exists(context.Background(), b.RedisRankKey(2)+"-preset").Result(); e != 0 {
		t.Fatal("拒绝后临时键没清掉")
	}
}

// 当前届【为空】时允许预置——结算窗口贴日初时,换届那一刻新一届已经是当前届了,
// 用「必须是未来届」去卡会把这条正常路径一并否掉
func TestZPresetAllowsEmptyCurrentCycle(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc) //zStmt = 第 1 届,且榜是空的
	n, err := b.ZPreset(1, presetMembers(30))
	if err != nil {
		t.Fatalf("当前届为空时应允许预置,实际 %v", err)
	}
	if n != 30 {
		t.Fatalf("应写入 30 条,实际 %v", n)
	}
}

// 当前届【非空】时必须拒绝:放行等于把正在用的榜整体重排
func TestZPresetRejectsLiveCurrentCycle(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	if err := b.ZAdd(1, "live", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ZPreset(1, presetMembers(50)); !errors.Is(err, ErrPresetNotEmpty) {
		t.Fatalf("应返回 ErrPresetNotEmpty,实际 %v", err)
	}
	n, _ := client.ZCard(context.Background(), b.RedisRankKey(1)).Result()
	if n != 1 {
		t.Fatalf("拒绝后当前届不该被动到,实际 %v 条", n)
	}
}

// 休战期照样能预置——预置本来就是要在冻结窗口里做的事
func TestZPresetIgnoresTruce(t *testing.T) {
	setupRedis(t)
	b := NewBucket("zpresettruce", "zpresettruce", 0, ScoreUnlimited, SortTypeAsc, &truceHandle{on: true}, WithoutTiebreak())
	b.zStmt.Store(NewStatement(1, 0, 0))
	t.Cleanup(func() {
		client.Del(context.Background(), b.RedisRankKey(1), b.RedisRankKey(2))
	})
	//先确认这个榜确实在休战:常规写入被拦
	if err := b.ZAdd(1, "x", 1); !errors.Is(err, ErrTruce) {
		t.Fatalf("前置条件不成立,ZAdd 应被休战拦下,实际 %v", err)
	}
	if _, err := b.ZPreset(2, presetMembers(10)); err != nil {
		t.Fatalf("预置不该被休战拦下,实际 %v", err)
	}
	if n, _ := client.ZCard(context.Background(), b.RedisRankKey(2)).Result(); n != 10 {
		t.Fatalf("第 2 届应有 10 条,实际 %v", n)
	}
}

// 空成员列表是调用方的错:预置空榜没有意义,且会让 RENAME 失败
func TestZPresetRejectsEmpty(t *testing.T) {
	setupRedis(t)
	b := newSwapBucket(t, SortTypeAsc)
	if _, err := b.ZPreset(2, nil); err == nil {
		t.Fatal("空成员列表应报错")
	}
}
