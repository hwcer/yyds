package player

import "github.com/hwcer/updater/operator"

// Pending 待发送的 Operator 暂存区。
//
// 装的是「已经 Submit 出来、但没能随当次回包发出去」的操作：跨玩家场景
// (Context.GetPlayer / Mutex.Lock / 批量锁本身) 让出自己这一侧的锁之前必须先 Submit，
// 否则改动会丢；而那时回包还没成形，只能先存这里，等回到自己这边时由
// context/service.go 的 Pull() 取走、随回包一起下发。
//
// 🔴 不要用 Updater.Dirty(...) 代替本类型：那是把 op 放回 Updater.dirty，而
// Updater.Release() 会把 u.dirty 里的每个 op 归还 sync.Pool 并置 nil —— 数据当场丢失，
// 且 op 可能被后续请求取走复用导致串数据。本类型是独立切片，不受 Release 影响。
//
// 早先本类型叫 Dirty，与 Player 上的同名字段一起遮蔽了嵌入的 Updater.Dirty 方法，
// 逼得调用方写 p.Updater.Dirty(...) 才能拿到 updater 那个。改名后遮蔽消失。
type Pending struct {
	dict []*operator.Operator
}

func (d *Pending) Push(opts ...*operator.Operator) {
	d.dict = append(d.dict, opts...)
}

func (d *Pending) Pull() []*operator.Operator {
	r := d.dict
	d.dict = nil
	return r
}
