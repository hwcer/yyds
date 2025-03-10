package gateway

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosrpc/xserver"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
	"strings"
)

var Service = xserver.Service(options.ServiceTypeGate)

func init() {
	Register(send)
	Register(broadcast)
	//Register(rooms.Broadcast, "room/broadcast")
}

// Register 注册协议，用于服务器推送消息
func Register(i any, prefix ...string) {
	if err := Service.Register(i, prefix...); err != nil {
		logger.Fatal("%v", err)
	}
}

func send(c *xshare.Context) any {
	uid := c.GetMetadata(options.ServiceMetadataUID)
	guid := c.GetMetadata(options.ServiceMetadataGUID)
	//logger.Debug("推送消息:%v  %v  %v", c.GetMetadata(rpcx.MetadataMessagePath), uid, string(c.Payload()))
	p := players.Players.Get(guid)
	//sock := Sockets.Socket(uid)
	if p == nil {
		logger.Debug("用户不在线,消息丢弃,UID:%v GUID:%v", uid, guid)
		return nil
	}
	if uid != "" {
		if id := p.GetString(options.ServiceMetadataUID); id != "" && id != uid {
			logger.Debug("用户UID不匹配,UID:%v GUID:%v", uid, guid)
			return nil
		}
	}

	mate := c.Metadata()
	if _, ok := mate[options.ServicePlayerLogout]; ok {
		players.Delete(p)
		return nil
	}
	path := c.GetMetadata(options.ServiceMessagePath)
	Emitter.emit(EventTypeMessage, p, path, mate)
	sock := players.Players.Socket(p)
	logger.Debug("推送消息  GUID:%v PATH:%v BODY：%s", guid, path, c.Bytes())
	if sock == nil {
		return nil
	}
	CookiesUpdate(mate, p)
	if len(path) == 0 {
		return nil //仅仅设置信息，不需要发送
	}

	if err := sock.Send(path, c.Bytes(), mate); err != nil {
		return err
	}
	return nil
}

func broadcast(c *xshare.Context) any {
	path := c.GetMetadata(options.ServiceMessagePath)
	logger.Debug("广播消息:%v", path)
	mate := c.Metadata()
	ignore := c.GetMetadata(options.ServiceMessageIgnore)
	ignoreMap := make(map[string]struct{})
	if ignore != "" {
		arr := strings.Split(ignore, ",")
		for _, v := range arr {
			ignoreMap[v] = struct{}{}
		}
	}

	players.Range(func(p *session.Data) bool {
		uid := p.GetString(options.ServiceMetadataUID)
		if _, ok := ignoreMap[uid]; ok {
			return true
		}
		CookiesUpdate(mate, p)
		Emitter.emit(EventTypeMessage, p, path, mate)
		if sock := players.Socket(p); sock != nil {
			_ = sock.Send(path, c.Bytes(), mate)
		}
		return true
	})
	return nil
}
