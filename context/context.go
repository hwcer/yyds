package context

import (
	"context"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/xclient"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players/player"
	"github.com/smallnest/rpcx/client"
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
		doc := this.Player.Document(options.ITypeRole)
		return doc.Get("guid").(string)
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

// Gateway 网关地址
func (this *Context) Gateway() string {
	var code uint64
	if this.Player != nil {
		code = this.Player.Gateway
	} else {
		meta := values.Metadata(this.Metadata())
		code = uint64(meta.GetInt64(options.ServicePlayerGateway))
	}
	if code == 0 {
		return ""
	}
	return utils.IPv4Decode(code)
}

func (this *Context) Call(ctx context.Context, servicePath, serviceMethod string, args, reply any) (err error) {
	err = xclient.XCall(ctx, servicePath, serviceMethod, args, reply)
	if err != nil {
		logger.Debug("send servicePath:%v , serviceMethod:%v , err:%v", servicePath, serviceMethod, err)
	}
	return
}

func (this *Context) Async(ctx context.Context, servicePath, serviceMethod string, args any) (call *client.Call, err error) {
	return xclient.Async(ctx, servicePath, serviceMethod, args)
}

// Send 推送消息，必须长连接在线
func (this *Context) Send(path string, v any, req values.Metadata) {
	if req == nil {
		req = values.Metadata{}
	}
	req[options.ServiceMessagePath] = path
	if _, ok := req[binder.HeaderContentType]; !ok {
		req[binder.HeaderContentType] = this.Binder(binder.ContentTypeModRes).Name()
	}
	if this.Player != nil {
		this.Player.Send(v, req)
		return
	}
	req[binder.HeaderAccept] = binder.Json.Name()
	if _, ok := req[options.ServiceMetadataGUID]; !ok {
		req[options.ServiceMetadataGUID] = this.GUid()
	}

	if gateway := this.Gateway(); gateway != "" {
		req.Set(options.SelectorAddress, gateway)
	} else {
		logger.Alert("grpc gateway is nil")
	}
	if rid := this.GetMetadata(options.ServiceMetadataRequestId); rid != "" {
		req.Set(options.ServiceMetadataRequestId, rid)
	}
	_ = xclient.CallWithMetadata(req, nil, options.ServiceTypeGate, "send", v, nil)
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

func (this *Selector) Set(k, v string) {
	name := options.GetServiceSelectorAddress(k)
	this.SetMetadata(name, v)
}

func (this *Selector) Remove(k string) {
	name := options.GetServiceSelectorAddress(k)
	this.SetMetadata(name, "")
}
