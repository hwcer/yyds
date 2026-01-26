package context

import (
	"reflect"
	"runtime/debug"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc"
	"github.com/hwcer/cosrpc/server"
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players"
	"github.com/hwcer/yyds/players/player"
)

/*
所有接口都必须已经登录
使用updater时必须使用playerHandle.data()来获取updater
*/

var Service = server.Service(options.ServiceTypeGame, handlerCaller, handlerFilter)
var Serialize func(c *Context, reply *Message) ([]byte, error) = serializeDefault

type filterCaller interface {
	Caller(node *registry.Node, c *Context) interface{}
}

func NewService(name string) *registry.Service {
	return server.Service(name, handlerCaller, handlerFilter)
}

func Register(i interface{}, prefix ...string) {
	var arr []string
	if gwcfg.Gateway.Prefix != "" {
		arr = append(arr, gwcfg.Gateway.Prefix)
	}
	if len(prefix) > 0 {
		arr = append(arr, prefix...)
	} else {
		arr = append(arr, "%v")
	}
	if err := Service.Register(i, arr...); err != nil {
		logger.Fatal("%v", err)
	}
}

// RegisterPrivate 注册只有内部机器才能访问的接口,用户无法通过网关访问
func RegisterPrivate(i interface{}, prefix ...string) {
	var arr []string
	if len(prefix) > 0 {
		arr = append(arr, prefix...)
	} else {
		arr = append(arr, "%v")
	}
	if err := Service.Register(i, arr...); err != nil {
		logger.Fatal("%v", err)
	}
}

var handlerFilter server.HandlerFilter = func(node *registry.Node) bool {
	if node.IsFunc() {
		_, ok := node.Method().(func(*Context) interface{})
		return ok
	} else if node.IsMethod() {
		t := node.Value().Type()
		if t.NumIn() != 2 || t.NumOut() != 1 {
			return false
		}
		return true
	} else {
		if _, ok := node.Binder().(filterCaller); !ok {
			v := reflect.Indirect(reflect.ValueOf(node.Binder()))
			logger.Debug("[%v]未正确实现Caller方法,会影响程序性能", v.Type().String())
		}
		return true
	}
}

var handlerCaller server.HandlerCaller = func(node *registry.Node, sc *cosrpc.Context) (reply any, err error) {
	c := &Context{Context: sc}
	path := c.ServiceMethod()
	if !gwcfg.HasServiceMethod(path) {
		return c.handle(node) //内网通信不启用玩家数据
	}

	path = gwcfg.TrimServiceMethod(path)
	auth := c.Permission()

	//l, m := c.GetMetadata(gwcfg.ServiceMetadataApi)

	if auth == gwcfg.OAuthTypeNone {
		return c.handle(node)
	}
	if auth == gwcfg.OAuthTypeOAuth {
		if guid := c.GetMetadata(gwcfg.ServiceMetadataGUID); guid == "" {
			return nil, errors.ErrLogin
		}
		return c.handle(node)
	}

	uid := c.Uid()
	if uid == "" {
		return nil, errors.ErrNotSelectRole
	}
	if auth == gwcfg.OAuthTypeSelect {
		return c.handle(node)
	}

	err = players.Get(uid, func(p *player.Player) error {
		if p == nil {
			return errors.ErrLogin
		}
		if p.Ban {
			return errors.ErrRoleIsBan
		}
		if p.Error != nil {
			return p.Error
		}
		c.Player = p
		c.Player.KeepAlive(c.Unix())
		//尝试重新上线
		meta := values.Metadata(c.Metadata())
		if c.Player.Status != player.StatusConnected {
			if e := players.Connected(p, meta); e != nil {
				return e
			}
		} else if gate := meta.GetUint64(gwcfg.ServiceMetadataGateway); gate != p.Gateway {
			return errors.ErrReplaced
		}
		if options.Setting.Renewal != "" && c.Player.Login < times.Daily(0).Now().Unix() && path != options.Setting.Renewal {
			return errors.ErrNeedResetSession
		}
		//重发
		if rid := meta.GetInt32(gwcfg.ServiceMetadataRequestId); rid > 0 && c.Player != nil {
			if c.Player.Message == nil {
				c.Player.Message = &player.Message{}
			}
			if c.Player.Message.Id == rid {
				reply = c.Player.Message.Data
				return nil
			}
			defer func() {
				c.Player.Message.Id = rid
				c.Player.Message.Data = reply.([]byte)
			}()
		}
		reply, err = c.handle(node)
		return err
	})
	if err != nil {
		return err, nil
	}
	c.Player = nil
	if c.Next != nil {
		c.Next()
	}

	return
}

func serializeDefault(c *Context, r *Message) ([]byte, error) {
	if r.Code == 0 && r.Data == nil {
		return nil, nil
	}
	b := c.Binder(binder.ContentTypeModRes)
	return b.Marshal(r)
}

func (c *Context) handle(node *registry.Node) (any, error) {
	r := c.caller(node)
	return Serialize(c, r)
}

func (c *Context) caller(node *registry.Node) (r *Message) {
	defer func() {
		if v := recover(); v != nil {
			r = Errorf(500, "server error")
			logger.Trace("server error:%v\n%v", v, string(debug.Stack()))
		}
	}()

	var v interface{}
	if node.IsFunc() {
		m := node.Method().(func(*Context) interface{})
		v = m(c)
	} else if s, ok := node.Binder().(filterCaller); ok {
		v = s.Caller(node, c)
	} else {
		vs := node.Call(c)
		v = vs[0].Interface()
	}
	var err error
	//直接返回二进制不做任何处理
	if b, ok := v.([]byte); ok {
		if c.Player != nil {
			_, err = c.Player.Submit()
		}
		if err != nil {
			return Error(err)
		}
		return Parse(b)
	}

	r = Parse(v)
	r.Time = c.Now().UnixMilli()
	if r.Code == 0 && c.Player != nil {
		if r.Cache, err = c.Player.Submit(); err == nil {
			r.Dirty = c.Player.Dirty.Pull()
		} else {
			r = Error(err)
		}
	}
	return r
}
