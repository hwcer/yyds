package context

import (
	"context"
	"fmt"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/xclient"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players"
	"github.com/hwcer/yyds/players/player"
	"github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/share"
	"strings"
	"time"
)

type Context struct {
	*xshare.Context
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

//多用户批量锁操作,不需要交互的情况下直接注释或者删除此文件

// Locker 批量获取玩家锁
// handle 获取批量操作权限
// next   获取操作结束后是否需要回到玩家自身,

func (this *Context) Locker(uids []string, handle player.LockerHandle, args any, next ...func()) (any, error) {
	var p *player.Player
	var done []func()
	if this.Player != nil {
		p = this.Player
		this.Player = nil
		p.Release()
		if players.Options.AsyncModel == players.AsyncModelLocker {
			p.Unlock()
		}
		done = append(done, func() {
			if players.Options.AsyncModel == players.AsyncModelLocker {
				p.Lock()
			}
			p.Reset()
			this.Player = p
		})
	}
	done = append(done, next...)
	return players.Locker(uids, handle, args, done...)
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

func (this *Context) AsyncWithPlayer(uid string, serviceMethod string, args any) (call *client.Call, err error) {
	u := &uuid.UUID{}
	if err = u.Parse(uid, uuid.BaseSize); err != nil {
		return nil, err
	}
	req := map[string]string{}
	req[options.SelectorServerId] = fmt.Sprintf("%d", u.GetShard())
	ctx := context.WithValue(context.Background(), share.ReqMetaDataKey, req)
	return xclient.Async(ctx, options.ServiceTypeGame, serviceMethod, args)
}

// Send 推送消息，必须长连接在线
func (this *Context) Send(path string, v any, req values.Metadata) {
	req = this.NewSender(path, req)
	if this.Player != nil {
		this.Player.Send(v, req)
		return
	}
	if _, ok := req[options.ServiceMetadataGUID]; !ok {
		req[options.ServiceMetadataGUID] = this.GUid()
	}

	if gateway := this.Gateway(); gateway != "" {
		req.Set(options.SelectorAddress, gateway)
	} else {
		logger.Alert("grpc gateway is nil")
	}

	if err := xclient.CallWithMetadata(req, nil, options.ServiceTypeGate, "send", v, nil); err != nil {
		logger.Error(err)
	}
}

func (this *Context) NewSender(path string, req values.Metadata) values.Metadata {
	if req == nil {
		req = values.Metadata{}
	}
	req[options.ServiceMessagePath] = path
	if _, ok := req[binder.HeaderContentType]; !ok {
		req[binder.HeaderContentType] = this.Binder(binder.ContentTypeModRes).Name()
	}
	if _, ok := req[options.ServiceMetadataRequestId]; !ok {
		if rid := this.GetMetadata(options.ServiceMetadataRequestId); rid != "" {
			req.Set(options.ServiceMetadataRequestId, rid)
		}
	}
	//如果 socket id存在，优先使用SOCKET ID推送消息
	if sockId := this.GetMetadata(options.ServiceSocketId); sockId != "" {
		req.Set(options.ServiceSocketId, sockId)
	}

	return req
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

func (this *Channel) Name(s ...string) string {
	return strings.Join(s, ".")
}

// Join 加入频道
func (this *Channel) Join(name ...string) {
	this.SetMetadata(options.ServicePlayerRoomJoin, this.Name(name...))
}

// Leave  退出频道
func (this *Channel) Leave(name ...string) {
	this.SetMetadata(options.ServicePlayerRoomLeave, this.Name(name...))
}

// Broadcast  频道广播
func (this *Channel) Broadcast(path string, args any, name ...string) {
	req := values.Metadata{}
	req[binder.HeaderContentType] = binder.Protobuf.String()
	req[options.ServiceMessagePath] = path
	req[options.ServiceMessageRoom] = this.Name(name...)
	if err := xclient.CallWithMetadata(req, nil, options.ServiceTypeGate, "broadcast", args, nil); err != nil {
		logger.Error(err)
	}
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
