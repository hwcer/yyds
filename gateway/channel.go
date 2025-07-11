package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/gateway/channel"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
)

func init() {
	Register(&channelHandle{}, "channel", "%m")
	channel.SendMessage = func(p *session.Data, path string, data []byte) {
		if sock := players.Socket(p); sock != nil {
			_ = sock.Send(0, path, data)
		}
	}
}

// 内部接口，游戏服务器广播
type channelHandle struct{}

func (this channelHandle) Broadcast(c *xshare.Context) any {
	path := c.GetMetadata(options.ServiceMessagePath)
	name := c.GetMetadata(options.ServiceMessageRoom)
	room := channel.Get(name)
	if room == nil {
		logger.Debug("房间不存在,room:%s  path:%s", name, path)
		return nil
	}
	room.Broadcast(path, c.Bytes())
	logger.Debug("频道广播,room:%s  path:%s", name, path)

	return nil
}

// Delete 删除一个频道
func (this channelHandle) Delete(c *xshare.Context) any {
	if name := c.GetMetadata(options.ServiceMessageRoom); name != "" {
		channel.Delete(name)
	}
	return nil
}
