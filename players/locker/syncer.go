package locker

import (
	"sync"

	"github.com/hwcer/yyds/players/player"
)

func newPlayer(uid string, test bool) *player.Player {
	p := player.New(uid, test)
	p.Syncer = NewSyncer()
	return p
}

type Syncer struct {
	sync.Mutex
}

func NewSyncer() player.Syncer {
	return &Syncer{}
}

func (m *Syncer) Close() {}
