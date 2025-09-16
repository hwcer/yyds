package context

import (
	"time"

	"github.com/hwcer/cosrpc"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players/player"
)

type Context struct {
	*cosrpc.Context
	Player *player.Player
}

// Uid 角色ID
func (this *Context) Uid() string {
	if this.Player != nil {
		return this.Player.Uid()
	}
	if r := this.GetMetadata(options.ServiceMetadataUID); r != "" {
		return r
	}
	return ""
}

// GUid 账号ID
func (this *Context) GUid() string {
	if this.Player != nil {
		return this.Player.Guid()
	}
	if r := this.GetMetadata(options.ServiceMetadataGUID); r != "" {
		return r
	}
	return ""
}

func (this *Context) Now() time.Time {
	if this.Player != nil {
		return this.Player.Now()
	}
	return time.Now()
}

func (this *Context) Unix() int64 {
	if this.Player != nil {
		return this.Player.Unix()
	}
	return time.Now().Unix()
}

// Milli 毫秒
func (this *Context) Milli() int64 {
	if this.Player != nil {
		return this.Player.Milli()
	}
	return time.Now().UnixMilli()
}
