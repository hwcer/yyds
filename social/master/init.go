package master

import (
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/xclient"
	"github.com/hwcer/cosweb"
	"github.com/hwcer/cosweb/middleware"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/social/model"
)

var db = model.DB()
var Server = cosweb.New() //默认服务器

var Service = Server.Service("")

func Start() (err error) {
	if model.Options.Address == "" {
		logger.Alert("social address is empty")
		return nil
	}
	access := middleware.NewAccessControlAllow()
	access.Origin("*")
	access.Methods("GET", "POST", "OPTIONS")
	Server.Use(access.Handle)
	Server.Register("/*", proxy)

	if err = Server.Listen(model.Options.Address); err == nil {
		logger.Trace("social server started success:%v", model.Options.Address)
	} else {
		logger.Alert("social server started error:%v", err)
	}
	return
}

func proxy(c *cosweb.Context, next cosweb.Next) error {
	sid := c.GetString("sid", cosweb.RequestDataTypeQuery)
	if sid == "" {
		return c.JSON(values.Error("sid is empty"))
	}
	req := values.Metadata{}
	req[options.SelectorServerId] = sid
	reply := make([]byte, 0)
	buffer, err := c.Buffer()
	if err != nil {
		return err
	}
	err = request(sid, c.Request.URL.Path, buffer.Bytes(), req, nil, &reply)
	if err != nil {
		return c.JSON(values.Error(err))
	}

	return c.Bytes(cosweb.ContentTypeApplicationJSON, reply)
}

// request rpc转发,返回实际转发的servicePath
func request(sid, path string, args []byte, req, res values.Metadata, reply any) (err error) {
	req[options.ServiceMetadataServerId] = sid
	req[binder.HeaderContentType] = binder.Json.Name()
	err = xclient.CallWithMetadata(req, res, options.ServiceTypeGame, path, args, reply)
	return
}

// Broadcast 网关广播消息
func Broadcast(path string, v any, req values.Metadata) error {
	if req == nil {
		req = values.Metadata{}
	}
	req.Set(binder.HeaderAccept, binder.Json.Name())
	req.Set(binder.HeaderContentType, options.Options.Binder)
	req.Set(options.ServiceMessagePath, path)
	ctx, cancel := xclient.WithTimeout(req, nil)
	defer cancel()

	return xclient.Broadcast(ctx, options.ServiceTypeGate, "broadcast", v, nil)
}
