package gateway

import (
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/gateway/rooms"
	"github.com/hwcer/yyds/options"
)

type channel struct{}

func (this channel) Broadcast(c *xshare.Context) any {
	path := c.GetMetadata(options.ServiceMessagePath)
	name := c.GetMetadata(options.ServiceMessageRoom)
	room := rooms.Get(name)
	if room == nil {
		logger.Debug("房间不存在,room:%s  path:%s", name, path)
		return nil
	}
	room.Broadcast(path, c.Bytes())
	logger.Debug("频道广播,room:%s  path:%s", name, path)

	return nil
}

func (this channel) Delete(c *xshare.Context) any {
	if name := c.GetMetadata(options.ServiceMessageRoom); name != "" {
		rooms.Delete(name)
	}
	return nil
}
