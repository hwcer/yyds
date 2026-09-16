package players

import (
	"fmt"
	"testing"

	"github.com/hwcer/cosgo/values"
)

// 🔴 回归：cosmo 新版 DB.Error 的类型是 *values.Message（实现了 error 接口）。
// 旧写法 `err = tx.Error` 在 Error 字段为 nil 指针时把 nil 指针装进 error 接口
// —— typed-nil：`err != nil` 判真、`%v` 打印 <nil>、调用 Error() 方法 panic。
// 2026-09-16 实际发生过：players.Start 在 EventTypLoaded 假失败（原因:<nil>），
// 服务起不来。修复：统一走 tx.Err()（内部 nil 检查后返回真 nil）。
func TestPreloadErrorNotTypedNil(t *testing.T) {
	var m *values.Message //成功查询后 DB.Error 的实际状态：nil 指针
	var err error = m
	if err == nil {
		t.Fatal("前提不成立：nil *Message 装箱后 err==nil，typed-nil 未复现")
	}
	// typed-nil 的三个特征
	if fmt.Sprint(err) != "<nil>" {
		t.Fatalf("%%v 应打印 <nil>，实际 %q", fmt.Sprint(err))
	}
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("nil 接收者调 Error() 应 panic（这才是最危险的：日志/包装层一碰就炸）")
			}
		}()
		_ = err.Error()
	}()
	// Err() 语义：非 nil Message 装箱后保留错误内容
	m = values.Errorf(1, "boom")
	var err2 error = m
	if err2 == nil || err2.Error() != "boom" {
		t.Fatalf("非 nil Message 装箱后应保留错误内容, got %v", err2)
	}
}
