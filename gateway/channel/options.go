package channel

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/logger"
)

var SendMessage = func(p *session.Data, path string, data []byte) {
	logger.Alert("channel SendMessage is nil")
}
