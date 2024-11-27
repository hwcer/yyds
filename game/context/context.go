package context

import (
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/game/players/player"
	"strconv"
	"time"
)

type Context struct {
	*xshare.Context
	Time   time.Time
	Player *player.Player
}

func (this *Context) reset() {
	if this.Player != nil {
		this.Time = this.Player.Time
	} else {
		this.Time = time.Now()
	}
	return
}

func (this *Context) release() {
	this.Context = nil
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
