package rank

import (
	"math"
	"testing"
	"time"
)

// newStmt 构造一个"已过 elapsed 秒、总时长 d 秒"的 Statement。
// elapsed 允许为负或超过 d,用于覆盖 formatScore 里的两个钳位分支。
func newStmt(elapsed, d int64) *Statement {
	now := time.Now().Unix()
	return &Statement{zTime: now - elapsed, zExpire: now - elapsed + d}
}

// 用真实的 formatScore/parseScore 验证:解码往返、同分内单调、跨分数区间不重叠
func TestFormatScore(t *testing.T) {
	scores := []int64{-1 << 40, -100, -5, -1, 0, 1, 100, 1 << 20, 1e9, 4294967295, 1e12, 1 << 45, 1<<52 - 1, 1<<53 - 1}
	cycles := []int64{86400, 604800, 2592000, 31536000} // 1天/7天/30天/1年
	types := []SortType{SortTypeDesc, SortTypeAsc}

	for _, d := range cycles {
		for _, zt := range types {
			w := &Bucket{zType: zt}
			pack := func(score, elapsed int64) float64 {
				return newStmt(elapsed, d).formatScore(w, score)
			}
			for _, sc := range scores {
				// 1. 解码往返
				for _, e := range []int64{0, 1, d / 2, d - 1} {
					if got := parseScore(pack(sc, e)); got != sc {
						t.Fatalf("d=%d type=%d score=%d elapsed=%d 解码=%d", d, zt, sc, e, got)
					}
				}
				// 2. 同分内单调:elapsed 增大,先达成者不得被反超
				prev := math.Inf(-1)
				if zt == SortTypeDesc {
					prev = math.Inf(1)
				}
				for e := int64(0); e < d; e += d/97 + 1 {
					p := pack(sc, e)
					if zt == SortTypeDesc && p > prev {
						t.Fatalf("d=%d desc score=%d elapsed=%d 反转", d, sc, e)
					}
					if zt == SortTypeAsc && p < prev {
						t.Fatalf("d=%d asc score=%d elapsed=%d 反转", d, sc, e)
					}
					prev = p
				}
				// 3. 跨分数:sc 的最优必须严格劣于 sc+1 的最劣(降序)
				if zt == SortTypeDesc && !(pack(sc, 0) < pack(sc+1, d-1)) {
					t.Fatalf("d=%d desc score=%d 与 %d 区间重叠", d, sc, sc+1)
				}
				if zt == SortTypeAsc && !(pack(sc, d-1) < pack(sc+1, 0)) {
					t.Fatalf("d=%d asc score=%d 与 %d 区间重叠", d, sc, sc+1)
				}
			}
		}
	}
}

// TestFormatScoreClamp 覆盖 elapsed 的两个钳位分支
//
// 这两个分支上一版测试从未执行到(pack 总是构造出 [0,d-1] 内的 elapsed),
// 删掉任一钳位测试都不会失败。
func TestFormatScoreClamp(t *testing.T) {
	const d = 604800
	for _, zt := range []SortType{SortTypeDesc, SortTypeAsc} {
		w := &Bucket{zType: zt}
		// zTime 在未来 → elapsed<0,应钳到0,等价于本届刚开始
		future := newStmt(-3600, d).formatScore(w, 42)
		if got := parseScore(future); got != 42 {
			t.Fatalf("type=%d 未来zTime 解码=%d", zt, got)
		}
		if start := newStmt(0, d).formatScore(w, 42); future != start {
			t.Fatalf("type=%d 未来zTime 未被钳到0: %v != %v", zt, future, start)
		}
		// elapsed 超过时长 → 应钳到 d-1,等价于本届最后一秒
		over := newStmt(d*3, d).formatScore(w, 42)
		if got := parseScore(over); got != 42 {
			t.Fatalf("type=%d 超期elapsed 解码=%d", zt, got)
		}
		if last := newStmt(d-1, d).formatScore(w, 42); over != last {
			t.Fatalf("type=%d 超期elapsed 未被钳到d-1: %v != %v", zt, over, last)
		}
	}
}

// 时长无效时不做 tiebreak
func TestFormatScoreNoDuration(t *testing.T) {
	w := &Bucket{zType: SortTypeDesc}
	now := time.Now().Unix() //必须取一次,两次调用可能跨秒导致时长变成 d+1
	for _, d := range []int64{0, 1, -100} {
		st := &Statement{zTime: now, zExpire: now + d}
		if got := st.formatScore(w, 12345); got != 12345 {
			t.Fatalf("d=%d 期望12345 得到%v", d, got)
		}
	}
}

// parseScore 必须饱和截断,不能让 float64→int64 越界翻转
func TestParseScoreSaturate(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{
		{math.MaxInt64, math.MaxInt64},
		{math.MinInt64, math.MinInt64},
		{1e300, math.MaxInt64},
		{-1e300, math.MinInt64},
		{math.Inf(1), math.MaxInt64},
		{math.Inf(-1), math.MinInt64},
		{0, 0},
		{-4.1, -5},
		{4.9, 4},
	}
	for _, c := range cases {
		if got := parseScore(c.in); got != c.want {
			t.Fatalf("parseScore(%v)=%d 期望%d", c.in, got, c.want)
		}
	}
}

// isMax 是尽力而为的前置过滤:设置了守门员就按方向过滤,未设置(0)则全部放行
func TestIsMax(t *testing.T) {
	for _, zt := range []SortType{SortTypeDesc, SortTypeAsc} {
		w := &Bucket{zType: zt, zMax: 100}
		st := &Statement{}
		// 未设置守门员:全部放行
		if !w.isMax(st, -999) || !w.isMax(st, 999) {
			t.Fatalf("type=%d 未设置守门员时应全部放行", zt)
		}
		// 设置守门员后按排序方向严格过滤:必须排在守门员之前才能进入
		st.zKeeper.Store(100)
		if zt == SortTypeDesc {
			if w.isMax(st, 100) || w.isMax(st, 99) || !w.isMax(st, 101) {
				t.Fatalf("desc 过滤错误")
			}
		} else if w.isMax(st, 100) || w.isMax(st, 101) || !w.isMax(st, 99) {
			t.Fatalf("asc 过滤错误")
		}
		// zMax==0(不限人数)时不过滤
		if unlimited := (&Bucket{zType: zt}); !unlimited.isMax(st, -999) {
			t.Fatalf("type=%d 不限人数时不应过滤", zt)
		}
	}
}

// zScore 的"不限制"必须用 ScoreUnlimited,0 是合法门槛
func TestScoreSentinel(t *testing.T) {
	unlimited := &Bucket{zType: SortTypeDesc, zScore: ScoreUnlimited}
	if !unlimited.isScore(math.MinInt64+1) || !unlimited.isScore(math.MaxInt64) {
		t.Fatal("ScoreUnlimited 应全部放行")
	}
	// 降序榜门槛0:只收非负分
	desc := &Bucket{zType: SortTypeDesc, zScore: 0}
	if desc.isScore(-1) || !desc.isScore(0) || !desc.isScore(1) {
		t.Fatal("desc 门槛0 应只收非负分")
	}
	// 升序榜门槛0:只收非正分
	asc := &Bucket{zType: SortTypeAsc, zScore: 0}
	if asc.isScore(1) || !asc.isScore(0) || !asc.isScore(-1) {
		t.Fatal("asc 门槛0 应只收非正分")
	}
}
