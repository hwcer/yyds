package player

import (
	"github.com/hwcer/logger"
	"strings"
)

type WorkerCreator func(player *Player) any

var workers = map[string]WorkerCreator{}

func Register(name string, creator WorkerCreator) {
	name = strings.ToLower(name)
	if workers[name] != nil {
		logger.Alert("player handle register already registered:%v", name)
	} else {
		workers[name] = creator
	}
}
