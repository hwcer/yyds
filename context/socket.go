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
	return client.Async(ctx, gwcfg.ServiceTypeGame, serviceMethod, args)
}

// Send 推送消息，必须长连接在线
//
// 有 Player 时交给 player.Send —— 它会用玩家对象上的值覆盖 UID/GUID/网关地址，
// 那份比从请求上下文取的更权威(角色可能刚换、网关可能刚重连)。
// 没有 Player 时(未选角的接口)直接用 NewSender 装好的那份。
func (this *Context) Send(path string, v any, req values.Metadata) {
	req = this.NewSender(path, req)
	if this.Player != nil {
		this.Player.Send(v, req)
		return
	}
	//网关按 socketId 与 GUID 二选一定位连接,两者都空就投递不出去 ——
	//必须在这里拦掉而不是发出去等它静默丢弃
	if req[gwcfg.ServiceMetadataGUID] == "" && req[gwcfg.ServiceMetadataSocketId] == "" {
		logger.Alert("消息推送失败,GUID 与 SocketId 均为空,path:%v", path)
		return
	}
	if req[selector.MetaDataAddress] == "" {
		logger.Alert("消息推送失败,网关地址为空,path:%v", path)
		return
	}
	if err := client.CallWithMetadata(req, nil, gwcfg.ServiceTypeGate, gwcfg.MessageSend, v, nil); err != nil {
		logger.Error("消息推送失败,guid:%v,path:%v,err:%v", req[gwcfg.ServiceMetadataGUID], path, err)
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
	//带上**发起这次请求的那条连接**。网关按 socketId 与 GUID 二选一定位,认 socketId 时
	//代次隔离是白送的:顶号或重连之后那条连接要么还在(推送与确认包一起回到它)、要么已销毁
	//(丢弃),绝不会改投新端。按 GUID 投才会——上一代连接的数据推给刚上来的另一个人,
	//而那次请求的确认包走的是请求自己的 socket,一次响应被劈成两半。
	if _, ok := req[gwcfg.ServiceMetadataSocketId]; !ok {
		if sockId := this.GetMetadata(gwcfg.ServiceMetadataSocketId); sockId != "" {
			req.Set(gwcfg.ServiceMetadataSocketId, sockId)
		}
	}
	//GUID / UID / 网关地址:网关定位连接与校验归属要用。有 Player 时 player.Send 会用
	//玩家对象上的值覆盖掉这里的(那份更权威),没有 Player 时就靠这里装的这份。
	if _, ok := req[gwcfg.ServiceMetadataGUID]; !ok {
		if guid := this.GUid(); guid != "" {
			req.Set(gwcfg.ServiceMetadataGUID, guid)
		}
	}
	if _, ok := req[gwcfg.ServiceMetadataUID]; !ok {
		if uid := this.Uid(); uid != "" {
			req.Set(gwcfg.ServiceMetadataUID, uid)
		}
	}
	if _, ok := req[selector.MetaDataAddress]; !ok {
		if gate := this.Gateway(); gate != "" {
			req.Set(selector.MetaDataAddress, gate)
		}
	}
	if _, ok := req[gwcfg.ServiceResponseFlag]; !ok {
		req[gwcfg.ServiceResponseFlag] = strconv.FormatUint(uint64(message.FlagNoreply), 10)
	}
	return req
}
