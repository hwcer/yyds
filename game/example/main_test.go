package main

import (
	"testing"
)

func TestName(t *testing.T) {
	t.Logf("0:%v", SubItems())
	t.Logf("1:%v", SubItems(100))
	t.Logf("2:%v", SubItems(200, 10000))
	t.Logf("3:%v", SubItems(300, 10000, 200000))
	t.Logf("%v", 1/5)
	t.Logf("%v", 4/5)
}

func SubItems(multi ...int32) [2]int32 {
	power := [2]int32{1, 0}
	if len(multi) > 0 {
		copy(power[0:2], multi)
	}
	return power
}
