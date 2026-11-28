package player

import (
	"testing"
	"time"

	"github.com/hwcer/cosgo/times"
)

// Expire 只给一个参数时它就是结束(时长);两个参数是 [开始,结束]
func TestTimesExpireArgs(t *testing.T) {
	var ts Times
	cases := []struct {
		name string
		t    int64
		args []int64
		want int64
	}{
		{"按天 30", 1, []int64{30}, times.Daily(30).Now().Unix()},
		{"按天 7", 1, []int64{7}, times.Daily(7).Now().Unix()},
		{"按周 1", 2, []int64{1}, times.Weekly(1).Now().Unix()},
		{"按月 1", 3, []int64{1}, times.Monthly(1).Now().Unix()},
		{"没有参数:按默认 1 天", 1, nil, times.Daily(1).Now().Unix()},
		{"[开始,结束]:取结束", 1, []int64{5, 30}, times.Daily(30).Now().Unix()},
		{"[开始,0]:结束按默认", 1, []int64{5, 0}, times.Daily(1).Now().Unix()},
		{"类型为 0", 0, []int64{30}, 0},
	}
	for _, c := range cases {
		if got, err := ts.Expire(c.t, c.args); err != nil || got != c.want {
			t.Errorf("Expire(%d, %v) = %d, %v; want %d", c.t, c.args, got, err, c.want)
		}
	}

	before := time.Now().Unix()
	got, err := ts.ExpireWithArray(4, 14400)
	after := time.Now().Unix()
	if err != nil || got < before+14400 || got > after+14400 {
		t.Errorf("ExpireWithArray(4, 14400) = %d, %v; want %d~%d", got, err, before+14400, after+14400)
	}
}
