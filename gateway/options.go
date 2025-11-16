package gateway

import (
	"fmt"
	"strings"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
	"github.com/hwcer/yyds/gateway/channel"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
)

func init() {
	cosgo.On(cosgo.EventTypStarted, func() error {
		//设置登录权限
		if Options.G2SOAuth != "" {
			servicePath, serviceMethod, err := Options.Router(Options.G2SOAuth, nil)
			if err != nil {
				return err
			}
			options.OAuth.Set(servicePath, serviceMethod, options.OAuthTypeOAuth)
		}
		//监控登录信息
		session.OnRelease(func(data *session.Data) {
			_ = players.Delete(data)
			channel.Release(data)
		})
		return nil
	})
}

var Options = struct {
	Router      router                                                        //路由处理规则
	C2SOAuth    string                                                        //网关登录
	G2SOAuth    string                                                        //游戏服登录验证,网关登录登录成功后继续使用GUID去游戏服验证,留空不进行验证
	S2CSecret   func(sock *cosnet.Socket, secret string)                      //登录成功时给客户端发送秘钥,空值不处理
	S2CReplaced func(sock *cosnet.Socket, address string)                     //被顶号时给客户端发送的顶号提示,空值不处理
	Request     func(player *session.Data, path string, meta values.Metadata) //网关转发消息时
	Response    func(player *session.Data, path string, meta values.Metadata) //推送数据时，不包括广播
	Serialize   func(c Context, reply any) ([]byte, error)
	Access      func(player *session.Data, token string, reply []byte) []byte //游戏端触发登录时用于处理登录信息
}{
	Router:    defaultRouter,
	C2SOAuth:  "oauth",
	Response:  defaultResponse,
	Serialize: defaultSerialize,
}

type router func(path string, req values.Metadata) (servicePath, serviceMethod string, err error)

// Router 默认路由处理方式
var defaultRouter router = func(path string, req values.Metadata) (servicePath, serviceMethod string, err error) {
	path = strings.TrimPrefix(path, "/")
	i := strings.Index(path, "/")
	if i < 0 {
		err = values.Errorf(404, "page not found")
		return
	}
	servicePath = registry.Formatter(path[0:i])
	serviceMethod = registry.Formatter(path[i:])
	return
}

type Context interface {
	Accept() binder.Binder
}

func defaultResponse(player *session.Data, path string, meta values.Metadata) {
	if _, ok := meta[options.ServiceMetadataRequestId]; !ok {
		i := player.Atomic()
		meta[options.ServiceMetadataRequestId] = fmt.Sprintf("%d", -i)
	}
}

func defaultSerialize(c Context, reply any) ([]byte, error) {
	b := c.Accept()
	v := values.Parse(reply)
	return b.Marshal(v)
}
