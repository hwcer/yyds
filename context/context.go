package context

import (
	"strconv"
	"time"

	"github.com/hwcer/cosrpc"
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/yyds/players/player"
)

type Context struct {
	*cosrpc.Context
	Next   func()
	Player *player.Player
}

// Uid 角色ID
func (this *Context) Uid() string {
	if this.Player != nil {
		return this.Player.Uid()
	}
	if r := this.GetMetadata(gwcfg.ServiceMetadataUID); r != "" {
		return r
	}
	return ""
}

// GUid 账号ID
func (this *Context) GUid() string {
	if this.Player != nil {
		return this.Player.Guid()
	}
	if r := this.GetMetadata(gwcfg.ServiceMetadataGUID); r != "" {
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
func (this *Context) OAuth() gwcfg.OAuthType {
	auth := this.GetMetadata(gwcfg.ServiceMetadataAuthorize)
	if auth == "" {
		return gwcfg.OAuthTypeNone
	}
	l, err := strconv.Atoi(auth)
	if err != nil {
		return gwcfg.OAuthTypeNone
	}
	return gwcfg.OAuthType(l)
}
