package gateway

import (
	"strconv"
	"strings"

	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosnet"
	"github.com/hwcer/cosrpc"
	"github.com/hwcer/cosrpc/server"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/modules/gateway/players"
	"github.com/hwcer/yyds/options"
)

var Service = server.Service(options.ServiceTypeGate)

func init() {
	Register(send)
	Register(write)
	Register(broadcast)
}

// Register 注册协议，用于服务器推送消息
func Register(i any, prefix ...string) {
	if err := Service.Register(i, prefix...); err != nil {
		logger.Fatal("%v", err)
	}
}

// 仅仅 在登录接口本身 需要提前对SOCKET发送信息时使用
func write(c *cosrpc.Context) any {
	id := c.GetMetadata(options.ServiceSocketId)
	if id == "" {
		return c.Error("socket id not found")
	}
	path := c.GetMetadata(options.ServiceMessagePath)
	i, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		logger.Debug("Socket id error,消息丢弃,Socket:%s PATH:%s ", id, path)
		return nil
	}
	sock := cosnet.Get(i)
	if sock == nil {
		logger.Debug("长链接不在线,消息丢弃,Socket:%s PATH:%s ", id, path)
		return nil
	}
	if len(path) == 0 {
		return nil //仅仅设置信息，不需要发送
	}
	sock.Send(0, path, c.Bytes())
	return nil
}

// send 消息推送
func send(c *cosrpc.Context) any {
	uid := c.GetMetadata(options.ServiceMetadataUID)
	guid := c.GetMetadata(options.ServiceMetadataGUID)

	p := players.Get(guid)
	if p == nil {
		logger.Debug("用户不在线,消息丢弃,UID:%s GUID:%s", uid, guid)
		return nil
	}
	if uid != "" {
		if id := p.GetString(options.ServiceMetadataUID); id != "" && id != uid {
			logger.Debug("用户UID不匹配,UID:%s GUID:%s", uid, guid)
			return nil
		}
	}

	mate := c.Metadata()
	if _, ok := mate[options.ServicePlayerLogout]; ok {
		players.Delete(p)
		return nil
	}
	path := c.GetMetadata(options.ServiceMessagePath)
	if Options.Response != nil {
		Options.Response(p, path, mate)
	}
	sock := players.Socket(p)
	if sock == nil {
		logger.Debug("长链接不在线,消息丢弃,UID:%s GUID:%s PATH:%s ", uid, guid, path)
		return nil
	}
	CookiesUpdate(mate, p)
	if len(path) == 0 {
		return nil //仅仅设置信息，不需要发送
	}
	var rid int32
	if s, ok := mate[options.ServiceMetadataRequestId]; ok {
		i, _ := strconv.Atoi(s)
		rid = int32(i)
	}
	//logger.Debug("推送消息  GUID:%s RID:%d PATH:%s", guid, rid, path)
	sock.Send(rid, path, c.Bytes())
	return nil
}

// broadcast 全服广播
func broadcast(c *cosrpc.Context) any {
	path := c.GetMetadata(options.ServiceMessagePath)
	//logger.Debug("广播消息:%v", path)
	//mate := c.Metadata()
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
		//CookiesUpdate(mate, p)
		//Emitter.emit(EventTypeBroadcast, p, path, nil)
		if sock := players.Socket(p); sock != nil {
			sock.Send(0, path, c.Bytes())
		}
		return true
	})
	return nil
}
