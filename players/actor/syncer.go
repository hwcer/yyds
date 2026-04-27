package actor

import (
	"context"
	"sync/atomic"

	"github.com/hwcer/cosgo/scc"
	"github.com/hwcer/yyds/players/player"
)

func newPlayer(uid string) *player.Player {
	p := player.New(uid)
	p.Syncer = NewSyncer()
	return p
}

type Syncer struct {
	ch      chan func()
	holding chan struct{}
	once    atomic.Bool
}

func NewSyncer() player.Syncer {
	return &Syncer{ch: make(chan func(), 128)}
}

func (c *Syncer) start() {
	if c.once.CompareAndSwap(false, true) {
		scc.CGO(c.worker)
	}
}

func (c *Syncer) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case fn, ok := <-c.ch:
			if ok {
				fn()
			} else {
				return
			}
		}
	}
}

func (c *Syncer) Lock() {
	c.start()
	ready := make(chan struct{})
	c.holding = make(chan struct{})
	c.ch <- func() {
		close(ready)
		<-c.holding
	}
	<-ready
}

func (c *Syncer) Unlock() {
	close(c.holding)
}

func (c *Syncer) invoke(fn func() error) error {
	c.start()
	done := make(chan error, 1)
	c.ch <- func() {
		done <- fn()
	}
	return <-done
}

func (c *Syncer) Close() {
	if c.ch != nil {
		close(c.ch)
		c.ch = nil
	}
}
