package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosrpc/xserver"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
	"strconv"
	"strings"
)

var Service = xserver.Service(options.ServiceTypeGate)

func init() {
	Register(send)
	Register(&channel{})
	Register(broadcast)
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
	p := players.Players.Get(guid)
	//sock := Sockets.Socket(uid)
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
	Emitter.emit(EventTypeResponse, p, path, mate)
	sock := players.Players.Socket(p)
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
	logger.Debug("推送消息  GUID:%s RID:%d PATH:%s", guid, rid, path)
	if err := sock.Send(rid, path, c.Bytes()); err != nil {
		return err
	}
	return nil
}

func broadcast(c *xshare.Context) any {
	path := c.GetMetadata(options.ServiceMessagePath)
	logger.Debug("广播消息:%v", path)
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
			_ = sock.Send(0, path, c.Bytes())
		}
		return true
	})
	return nil
}
