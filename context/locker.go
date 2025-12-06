package context

import (
	"context"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/players"
	"github.com/hwcer/yyds/players/player"
)

// GetPlayer 操作其他玩家
func (this *Context) GetPlayer(c *Context, uid string, handle player.Handle) error {
	if c.Player != nil && c.Player.Uid() == uid {
		return handle(c.Player)
	}

	if c.Player != nil {
		p := c.Player
		cs, _ := p.Submit()
		p.Updater.Dirty(cs...)
		p.Release()
		p.Unlock()
		c.Player = nil
		defer func() {
			p.Lock()
			p.Reset()
			c.Player = p
		}()
	}

	err := players.Get(uid, handle)
	if err == nil || !errors.Is(err, errors.ErrNotOnline) {
		return err
	}
	//强制登录
	return players.Load(uid, true, handle)

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
	//var p *player.Player
	var done []func()
	var includingOneself bool
	for _, k := range uids {
		if k == this.ctx.Uid() {
			includingOneself = true
			break
		}
	}

	if p := this.ctx.Player; p != nil && includingOneself {
		this.ctx.Player = nil
		p.Release()
		if players.Options.AsyncModel == players.AsyncModelLocker {
			p.Unlock()
		}
		done = append(done, func() {
			if players.Options.AsyncModel == players.AsyncModelLocker {
				p.Lock()
			}
			p.Reset()
			this.ctx.Player = p
		})
	}
	done = append(done, next...)
	return players.Locker(uids, args, handle, done...)
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
		_, _ = players.Locker(uids, args, lh, done...)
	})
}
