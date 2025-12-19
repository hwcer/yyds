package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosrpc"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/modules/gateway/channel"
	"github.com/hwcer/yyds/modules/gateway/players"
	"github.com/hwcer/yyds/options"
)

func init() {
	Register(&channelHandle{}, "channel", "%m")
	channel.SendMessage = func(p *session.Data, path string, data []byte) {
		if sock := players.Socket(p); sock != nil {
			sock.Send(0, path, data)
		}
	}
}

// 内部接口，游戏服务器广播
type channelHandle struct{}

func (this channelHandle) Broadcast(c *cosrpc.Context) any {
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

// Delete 删除一个频道,如果path不为空，先使用path广播再删除
func (this channelHandle) Delete(c *cosrpc.Context) any {
	name := c.GetMetadata(options.ServiceMessageRoom)
	if name == "" {
		logger.Debug("频道名不能为空")
		return nil
	}
	room := channel.Get(name)
	if room == nil {
		logger.Debug("房间不存在,room:%s", name)
		return nil
	}

	if path := c.GetMetadata(options.ServiceMessagePath); path != "" {
		room.Broadcast(path, c.Bytes())
		logger.Debug("频道广播 name:%s  path:%s", name, path)
	}
	logger.Debug("删除频道 %s", name)
	channel.Delete(name)
	return nil
}
