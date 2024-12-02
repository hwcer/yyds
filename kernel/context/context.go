package context

import (
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/kernel/config"
	"github.com/hwcer/yyds/kernel/players/player"
	"strconv"
	"strings"
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

// GUid 账号ID
func (this *Context) GUid() string {
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

func (this *Context) Unix() int64 {
	return this.Time().Unix()
}

// Channel 频道操作器
func (this *Context) Channel() *Channel {
	return &Channel{Context: this}
}

// Selector 微服务设置器
func (this *Context) Selector() *Selector {
	return &Selector{Context: this}
}

type Channel struct {
	*Context
}

// Join 加入频道
func (this *Channel) Join(name ...string) {
	this.SetMetadata(options.ServicePlayerRoomJoin, strings.Join(name, "."))
}

// Leave  退出频道
func (this *Channel) Leave(name ...string) {
	this.SetMetadata(options.ServicePlayerRoomLeave, strings.Join(name, "."))
}

type Selector struct {
	*Context
}

// Set 设置微服务地址
func (this *Context) Set(k, v string) {
	name := options.GetServiceSelectorAddress(k)
	this.SetMetadata(name, v)
}

// Delete 删除微服务地址
func (this *Context) Delete(k string) {
	name := options.GetServiceSelectorAddress(k)
	this.SetMetadata(name, "")
}
