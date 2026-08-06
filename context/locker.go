package context

import (
	"context"

	"github.com/hwcer/cosgo/scc"
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
// 让出前必须 Submit,否则本次请求已产生的改动会丢;产生的 Operator 转存进 Player.Dirty,
// 等回到自己这边时随回包一起下发。
//
// 🔴 必须是 p.Dirty.Push 而不是 p.Updater.Dirty:后者是把 op 放回 Updater.dirty,
// 而紧接着的 p.Release() 会 `for _, op := range u.dirty { op.Release() }` 把它们
// 归还 sync.Pool 并置 nil —— 改动当场丢失,且 op 可能被后续请求取走复用导致串数据。
// Player.Dirty 是独立切片,不受 Updater.Release() 影响,由 context/service.go 的
// Dirty.Pull() 收走。对照 players/actor/channel.go 与 players/locker/locker.go,
// 那两处平行实现用的都是 Dirty.Push。
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
			p.Dirty.Push(cs...)
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
		err = players.Load(uid, handle)
	}
	return
}

// Mutex 玩家互斥锁，需要同时获得多个用户锁时使用
// 可以防止死锁，不需要手动解锁
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
func (this *Mutex) Async(uids []string, args any, handle player.AsyncHandle, done ...func()) {
	lh := func(locker player.Locker, args any) (any, error) {
		handle(locker, args)
		return nil, nil
	}
	scc.SGO(func(ctx context.Context) {
		_, _ = players.Locker("", uids, args, lh, done...)
	})
}
