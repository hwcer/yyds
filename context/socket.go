package context

import (
	"context"
	"strconv"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet/message"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/cosrpc/selector"
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/options"
	"github.com/smallnest/rpcx/share"
)

//长链接推送消息相关

// Gateway 网关地址
func (this *Context) Gateway() string {
	var code uint64
	if this.Player != nil {
		code = this.Player.Gateway
	} else {
		meta := values.Metadata(this.Metadata())
		code = uint64(meta.GetInt64(gwcfg.ServiceMetadataGateway))
	}
	if code == 0 {
		return ""
	}
	return utils.IPv4Decode(code)
}

func (this *Context) Call(ctx context.Context, servicePath, serviceMethod string, args, reply any) (err error) {
	err = client.XCall(ctx, servicePath, serviceMethod, args, reply)
	if err != nil {
		logger.Debug("send servicePath:%v , serviceMethod:%v , err:%v", servicePath, serviceMethod, err)
	}
	return
}

func (this *Context) Async(ctx context.Context, servicePath, serviceMethod string, args any) (call *client.Caller, err error) {
	return client.Async(ctx, servicePath, serviceMethod, args)
}

// AsyncWithPlayer 按 uid 路由到对应游戏服异步调用
//
// 两个隐含约束:
//   - 用的是 context.Background(),**不继承本次请求的 ctx**,取消与超时都传不下去
//   - metadata 只带 ServerId,不带 UID/GUID,所以目标服拿到的请求没有玩家身份,
//     只能调 OAuthTypeNone 级别的接口
func (this *Context) AsyncWithPlayer(uid string, serviceMethod string, args any) (call *client.Caller, err error) {
	u := &uuid.UUID{}
	if err = u.Parse(uid, uuid.BaseSize); err != nil {
		return nil, err
	}
	req := values.Metadata{}
	sid := strconv.FormatUint(u.GetShard(), 10)
	req.Set(selector.MetaDataServerId, sid)
	ctx := context.WithValue(context.Background(), share.ReqMetaDataKey, req)
	return client.Async(ctx, options.ServiceTypeGame, serviceMethod, args)
}

// Send 推送消息，必须长连接在线
//
// 有 Player 时交给 player.Send(它自己校验在线状态/网关/Binder/GUID);
// 没有 Player 时(未选角的接口)在这里手工组装,校验项与 player.Send 保持一致 ——
// 两条路径下发的 metadata 和失败时的可观测性必须一样,否则线上只有一条路会出问题、
// 另一条却查不出来。
func (this *Context) Send(path string, v any, req values.Metadata) {
	req = this.NewSender(path, req)
	if this.Player != nil {
		this.Player.Send(v, req)
		return
	}
	//网关按 GUID 或 SocketId 定位会话,两者都空就投递不出去 ——
	//必须在这里拦掉而不是发出去等它静默丢弃
	guid, ok := req[gwcfg.ServiceMetadataGUID]
	if !ok {
		if guid = this.GUid(); guid != "" {
			req[gwcfg.ServiceMetadataGUID] = guid
		}
	}
	if guid == "" && req[gwcfg.ServiceMetadataSocketId] == "" {
		logger.Alert("消息推送失败,GUID 与 SocketId 均为空,path:%v", path)
		return
	}
	gateway := this.Gateway()
	if gateway == "" {
		logger.Alert("消息推送失败,网关地址为空,guid:%v,path:%v", guid, path)
		return
	}
	req.Set(selector.MetaDataAddress, gateway)
	if err := client.CallWithMetadata(req, nil, gwcfg.ServiceName, gwcfg.MessageSend, v, nil); err != nil {
		logger.Error("消息推送失败,guid:%v,path:%v,err:%v", guid, path, err)
	}
}

func (this *Context) NewSender(path string, req values.Metadata) values.Metadata {
	if req == nil {
		req = values.Metadata{}
	}
	req[gwcfg.ServiceMessagePath] = path
	if _, ok := req[binder.HeaderContentType]; !ok {
		req[binder.HeaderContentType] = this.Binder(binder.HeaderAccept, binder.HeaderContentType).Name()
	}
	if _, ok := req[gwcfg.ServiceMetadataRequestId]; !ok {
		if rid := this.GetMetadata(gwcfg.ServiceMetadataRequestId); rid != "" {
			req.Set(gwcfg.ServiceMetadataRequestId, rid)
		}
	}
	//如果 socket id存在，优先使用SOCKET ID推送消息
	if sockId := this.GetMetadata(gwcfg.ServiceMetadataSocketId); sockId != "" {
		req.Set(gwcfg.ServiceMetadataSocketId, sockId)
	}
	if _, ok := req[gwcfg.ServiceResponseFlag]; !ok {
		req[gwcfg.ServiceResponseFlag] = strconv.FormatUint(uint64(message.FlagNoreply), 10)
	}
	return req
}
