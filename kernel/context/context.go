package context

import (
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/kernel/config"
	"github.com/hwcer/yyds/kernel/players/player"
	"strconv"
	"time"
)

type Context struct {
	*xshare.Context
	Player *player.Player
}

// Uid 角色ID
func (this *Context) Uid() uint64 {
	if this.Player != nil {
		return this.Player.Uid()
	}
	if r := this.GetMetadata(options.ServiceMetadataUID); r != "" {
		v, _ := strconv.ParseUint(r, 10, 64)
		return v
	}
	return 0
}

// Guid 账号ID
func (this *Context) Guid() string {
	if this.Player != nil {
		doc := this.Player.Document(config.ITypeRole)
		return doc.Get("guid").(string)
	}
	if r := this.GetMetadata(options.ServiceMetadataGUID); r != "" {
		return r
	}
	return ""
}

func (this *Context) Time() time.Time {
	if this.Player != nil {
		return this.Player.Time
	}
	return time.Now()
}

// SetService 设置微服务地址
func (this *Context) SetService(k, v string) {
	name := options.GetServiceSelectorAddress(k)
	this.SetMetadata(name, v)
}

// SetChannel 设置聊天频道
func (this *Context) SetChannel(k, v string) {
	name := options.GetPlayerMessageChannel(k)
	this.SetMetadata(name, v)
}
