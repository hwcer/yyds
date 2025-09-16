package context

import (
	"context"
	"fmt"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/client"
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
		code = uint64(meta.GetInt64(options.ServicePlayerGateway))
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

func (this *Context) AsyncWithPlayer(uid string, serviceMethod string, args any) (call *client.Caller, err error) {
	u := &uuid.UUID{}
	if err = u.Parse(uid, uuid.BaseSize); err != nil {
		return nil, err
	}
	req := map[string]string{}
	req[options.SelectorServerId] = fmt.Sprintf("%d", u.GetShard())
	ctx := context.WithValue(context.Background(), share.ReqMetaDataKey, req)
	return client.Async(ctx, options.ServiceTypeGame, serviceMethod, args)
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

	if err := client.CallWithMetadata(req, nil, options.ServiceTypeGate, "send", v, nil); err != nil {
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
