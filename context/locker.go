package context

import (
	"context"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players"
	"github.com/hwcer/yyds/players/player"
)

// GetPlayer 操作其他玩家
//
// 目标就是自己时直接回调,不做任何锁操作。
//
// 目标是别人时,**会先把自己这一侧结清并让出锁**再去锁对方,这是防死锁的关键:
// 两个请求互相操作对方时,若各自攥着自己的锁去抢对方的,就是标准的 ABBA。
// 让出前必须 Submit,否则本次请求已产生的改动会丢;产生的 Operator 转存进 Player.Pending,
// 等回到自己这边时随回包一起下发。
//
// 🔴 必须是 p.Pending.Push 而不是 p.Dirty(...):后者是 Updater 的方法,把 op 放回
// Updater.dirty,而紧接着的 p.Release() 会 `for _, op := range u.dirty { op.Release() }`
// 把它们归还 sync.Pool 并置 nil —— 改动当场丢失,且 op 可能被后续请求取走复用导致串数据。
// Player.Pending 是独立切片,不受 Updater.Release() 影响,由 context/service.go 的
// Pending.Pull() 收走。批量锁(players/batch.go 的 batch.Submit)用的也是 Pending.Push。
//
// 代价是一次 GetPlayer 会让自己的 Updater 走完整的 Submit → Release → Reset 周期:
// **中途会真的提交一次数据库**,并重新 Emit EventTypeReset。调用方要清楚这一点 ——
// 它不是一次轻量的"顺便看一眼别人"。
//
// defer 里 p.Lock() 之后**故意不解锁**:锁要还给调用方,handler 后续还要继续用
// c.Player,由最外层的 players.Get 统一释放。
func (this *Context) GetPlayer(uid string, handle player.Handle) (err error) {
	if this.Player != nil && this.Player.Uid() == uid {
		return handle(this.Player)
	}
	//解除自身锁
	if this.Player != nil {
		p := this.Player
		if cs, e := p.Submit(); e != nil {
			return e
		} else {
			p.Pending.Push(cs...)
		}
		p.Release()
		p.Unlock()
		this.Player = nil
		defer func() {
			p.Lock()
			p.Reset()
			this.Player = p
		}()
	}

	if err = players.Get(uid, handle); err != nil && errors.Is(err, errors.ErrNotOnline) {
		err = players.Load(uid, true, handle)
	}
	return
}

// Mutex 玩家互斥锁，需要同时获得多个用户锁时使用
// 可以防止死锁，不需要手动解锁
//
// 🔴 **拿到的玩家不一定在线，也不一定有数据**：批量锁只负责「取得操作权限」，
// 不在内存的玩家给的是**空壳**（p.Updater == nil），直接读写就是当场 nil panic，
// 需要数据请按需手动加载。完整契约见 player.Locker 接口上的说明。
func (this *Context) Mutex() *Mutex {
	return &Mutex{ctx: this}
}

type Mutex struct {
	ctx *Context
}

// Lock 批量获取玩家锁,同步等结果
//
//	uids    要取得的玩家;取锁阶段整批成败,任何一个取不到整个回调都不会执行
//	args    原样传给 handle
//	handle  取得权限后的回调,拿到的是 player.Locker(空壳契约见上面 Mutex 的说明)
//	next    回调结束、锁全部归还之后要执行的收尾,追加在"锁回自己"之后
//
// 🔴 `this.ctx.Player != nil` 就等于「我此刻持有它」,不需要另立标记。
//
// 这条不变式是被**刻意维护**的:让出锁的地方一律同时把 ctx.Player 置 nil ——
// GetPlayer 在 Unlock 后置 nil、defer 里重新 Lock 才还回去,本函数下面也是同一套。
// 反过来说,新增任何「中途让出锁」的代码路径,都必须同步把 ctx.Player 置 nil,
// 否则这条判据就不成立了。
//
// ⚠ 代价与 GetPlayer 相同:让出自身锁会让你的 Updater 走完整的 Submit → Release → Reset
// 周期,**中途会真的提交一次数据库**,并重新 Emit EventTypeRelease / EventTypeReset。
// 它不是一次轻量的"顺便锁一下别人"。
func (this *Mutex) Lock(uids []string, args any, handle player.LockerHandle, next ...func()) (any, error) {
	var done []func()
	var self *player.Player
	if p := this.ctx.Player; p != nil {
		self = p
		//🔴 让出自己那把锁之前**必须先 Submit**(契约见 player.Pending 的注释):
		//让出期间别人可能取得同一个玩家,而他结束时的 Release() 会把你本次请求攒着的 dirty
		//逐个归还 sync.Pool 并置 nil —— 改动当场丢失,op 还可能被后续请求取走复用导致串数据。
		//Submit 出来的 operator 转存进 Player.Pending(独立切片,不受 Release 影响),
		//回到自己这边时由 context/service.go 的 Pull() 收走、随回包一起下发。
		//
		//放在置 ctx.Player 之前:提交失败时调用方手上的 c.Player 原样可用,
		//不会留下一个"锁已让出、指针已置空"的半拉状态。
		cs, e := p.Submit()
		if e != nil {
			return nil, e
		}
		p.Pending.Push(cs...)

		this.ctx.Player = nil
		//🔴 必须让出自己这把锁:批量锁进去之后会逐个抢目标玩家的锁,
		//攥着自己的去抢别人的就是标准 ABBA。
		//Release/Reset 与 Submit 配套:让出等于走完一个请求边界,回来时重新开始 ——
		//少了 Reset,Release 清掉的 status/Error 不会复位,后续 Submit 的收敛循环可能整个跳过。
		p.Release()
		p.Unlock()
		done = append(done, func() {
			p.Lock()
			p.Reset()
			this.ctx.Player = p
		})
	}
	done = append(done, next...)
	reply, err := players.Locker(uids, args, handle, done...)
	//🔴 取锁整批失败时 done 不会执行,自己的锁还悬在让出状态:
	//	available() 在 newBatch 之前就拒绝、或 await 排队超时(超时 ⇔ 任务从未开跑,
	//	worker 那边会因 CAS 失败跳过它)—— 这两条路径 batch.call 根本不运行。
	//	不补的话,外层 players.Get 的 defer p.Unlock() 会解锁一把未锁的 mutex 当场 panic。
	//	ctx.Player != nil 说明 done 已执行过(失败也可能发生在 handle 内,那时 release 照跑),不能重复恢复
	if err != nil && self != nil && this.ctx.Player == nil {
		self.Lock()
		self.Reset()
		this.ctx.Player = self
	}
	return reply, err
}

// Async 异步获得锁,独立协程执行锁任务
// 使用场景:锁中任务和当前任务无任何关系时使用,避免当前业务响应超时
// 参数同 Lock
//
// 📌 与 Lock 不同,**不让出自己那把锁**,也不需要:取锁跑在另一个协程上,当前请求不等它,
// 构不成"攥着自己的去抢别人的"那种环。
//
// ⚠ **取锁阶段整批成败**：uids 里任何一个取不到锁（非法 uid / 拒绝态 / await 超时），
// 整个任务都不会执行 —— 包括那些本来没问题的玩家。调用方拿不到这个错误
// （fire-and-forget），所以下面必须留痕：不打日志的话，现象是"偶尔就是没同步"，
// 而服务端一行记录都没有。
func (this *Mutex) Async(uids []string, args any, handle player.AsyncHandle, done ...func()) {
	lh := func(locker player.Locker, args any) (any, error) {
		handle(locker, args)
		return nil, nil
	}
	scc.SGO(func(ctx context.Context) {
		if _, err := players.Locker(uids, args, lh, done...); err != nil {
			logger.Alert("Mutex.Async 取锁失败,任务未执行:uids=%v,err=%v", uids, err)
		}
	})
}
