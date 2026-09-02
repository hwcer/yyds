package players

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestMigratePostNeverBlocks 🔴 投递**绝不能阻塞** daemon。
//
// 这是执行池存在的全部意义:daemon 是单协程,它一卡,心跳判定、掉线判定、回收判定全部顺延。
// 队列满时正确的反应是放弃本轮投递 —— 那些玩家的状态没变,下一个 tick 会重新收集到。
func TestMigratePostNeverBlocks(t *testing.T) {
	old := migrate
	t.Cleanup(func() { migrate = old })
	migrate = make(chan func(), 2) //故意不起 worker:投满两个之后就该拒

	if !migratePost(func() {}) || !migratePost(func() {}) {
		t.Fatal("队列没满就拒绝投递")
	}
	done := make(chan bool, 1)
	go func() { done <- migratePost(func() {}) }()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("队列已满却报告投递成功,任务会被静默丢弃")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("投递被阻塞了:daemon 绝不能卡在这里")
	}
}

// TestMigratePostWithoutPool 没起 daemon 时(单元测试 / 无玩家容器的服务)投递应当报失败,
// 而不是往 nil channel 上一送就永久阻塞。
func TestMigratePostWithoutPool(t *testing.T) {
	old := migrate
	t.Cleanup(func() { migrate = old })
	migrate = nil

	done := make(chan bool, 1)
	go func() { done <- migratePost(func() {}) }()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("没有执行池时不该报告投递成功")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("往 nil channel 投递被永久阻塞")
	}
}

// TestMigrateCallRecovers 逐个任务 recover:一个玩家迁移时 panic,不能带走整个 worker。
//
// 🔴 搬进执行池之前这些动作跑在 daemon 里,由 worker() 那层 recover 兜着 —— 代价是当轮剩下的
// 迁移全部作废。按任务隔离之后,一个坏玩家只影响他自己;不隔离的话是 worker 协程直接消失,
// 剩下的玩家从此没人释放,而且悄无声息。
func TestMigrateCallRecovers(t *testing.T) {
	migrateCall(func() { panic("boom") }) //没被 recover 的话这里就把测试打挂了

	ran := false
	migrateCall(func() { ran = true })
	if !ran {
		t.Fatal("panic 之后后续任务应当照常执行")
	}
}

// TestReleaseAllEmpty 停服专用的并行释放:空列表直接返回,不能起协程也不能卡住。
func TestReleaseAllEmpty(t *testing.T) {
	done := make(chan struct{})
	go func() { releaseAll(nil); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("空列表卡住了")
	}
}

// TestMigrateProcessDrainsOnStop 🔴 收到停服信号必须先把队列排空再退。
//
// 直接 return 的话,队列里那些已经判定过、正等着执行的迁移就连同数据一起丢了 ——
// disconnect/offline 欠着业务的下线事件,released 欠着一次 BulkWrite,而且悄无声息。
func TestMigrateProcessDrainsOnStop(t *testing.T) {
	old := migrate
	t.Cleanup(func() { migrate = old })
	migrate = make(chan func(), 16)

	var n int32
	for i := 0; i < 5; i++ {
		migrate <- func() { atomic.AddInt32(&n, 1) }
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() //进入循环时 ctx 已经 Done,模拟"信号先到、队列还有货"

	migrateWaitGroup.Add(1) //migrateProcess 内部会 Done
	done := make(chan struct{})
	go func() { migrateProcess(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("排空后没有退出")
	}
	if got := atomic.LoadInt32(&n); got != 5 {
		t.Fatalf("停服信号到来时丢弃了队列里的任务:只执行了 %d/5", got)
	}
}

// TestMigrateDrain 停服兜底:池已经退出之后投进来的任务,由 shutdown 自己排空。
//
// ctx 触发时 daemon 可能正好在 worker() 里投递,而池的 worker 已经排空退出 ——
// 那批任务谁都不会碰。这条钉住 shutdown 会兜住它们。
func TestMigrateDrain(t *testing.T) {
	old := migrate
	t.Cleanup(func() { migrate = old })
	migrate = make(chan func(), 16)

	var n int32
	for i := 0; i < 3; i++ {
		migrate <- func() { atomic.AddInt32(&n, 1) }
	}
	migrateDrain()
	if got := atomic.LoadInt32(&n); got != 3 {
		t.Fatalf("兜底排空漏了任务:只执行了 %d/3", got)
	}
	migrateDrain() //空队列上幂等,不能阻塞
}

// TestMigrateDrainWithoutPool 没起过 daemon 时 migrate 为 nil,排空不能阻塞。
func TestMigrateDrainWithoutPool(t *testing.T) {
	old := migrate
	t.Cleanup(func() { migrate = old })
	migrate = nil

	done := make(chan struct{})
	go func() { migrateDrain(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("nil 队列上排空被阻塞")
	}
}
