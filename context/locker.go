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
// Pending.Pull() 收走。对照 players/actor/channel.go 与 players/locker/locker.go,
// 那两处平行实现用的都是 Pending.Push。
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

// Lock 批量获取玩家锁
// args  参数会传递给handle
// handle 获取批量操作后回调函数
// next   获取操作结束后是否需要回到玩家自身,

// 🔴 `this.ctx.Player != nil` 就等于「我此刻持有它」,不需要另立标记。
//
// 这条不变式是被**刻意维护**的:让出锁的地方一律同时把 ctx.Player 置 nil ——
// GetPlayer 在 Unlock 后置 nil、defer 里重新 Lock 才还回去,本函数下面也是同一套。
// 所以 self 的含义就是「**我已经在锁内,别再替我排一次队**」。
//
// 反过来说,新增任何「中途让出锁」的代码路径,都必须同步把 ctx.Player 置 nil,
// 否则这条判据就不成立了。
func (this *Mutex) Lock(uids []string, args any, handle player.LockerHandle, next ...func()) (any, error) {
	var done []func()
	var self string

	if p := this.ctx.Player; p != nil {
		self = p.Uid()
		this.ctx.Player = nil
		if players.Options.AsyncModel == players.AsyncModelLocker {
			p.Unlock()
		}
		done = append(done, func() {
			if players.Options.AsyncModel == players.AsyncModelLocker {
				p.Lock()
			}
			this.ctx.Player = p
		})
	}
	done = append(done, next...)
	return players.Locker(self, uids, args, handle, done...)
}

// Async 异步获得锁，独立协程执行锁任务
// 使用场景：锁中任务和当前任务无任何关系时使用
// 避免当前业务响应超时
// 参数同 Lock
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
		if _, err := players.Locker("", uids, args, lh, done...); err != nil {
			logger.Alert("Mutex.Async 取锁失败,任务未执行:uids=%v,err=%v", uids, err)
		}
	})
}
