package context

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players"
	"github.com/hwcer/yyds/players/player"
	"strings"
)

const (
	ServiceMethodOAuthName  = "_ServiceMethodOAuth"
	ServiceMethodOAuthValue = "1"
)

func caller(c *Context, node *registry.Node) any {
	var v interface{}
	if node.IsFunc() {
		m := node.Method().(func(*Context) interface{})
		v = m(c)
	} else if s, ok := node.Binder().(Caller); ok {
		v = s.Caller(node, c)
	} else {
		vs := node.Call(c)
		v = vs[0].Interface()
	}
	var err error
	//直接返回二进制不做任何处理
	if r, ok := v.([]byte); ok {
		if c.Player != nil {
			_, err = c.Player.Submit()
		}
		if err != nil {
			return Error(err)
		} else {
			return r
		}
	}

	r := Parse(v)
	r.Time = c.Now().UnixMilli()
	if l := c.GetValue(ServiceMethodOAuthName); l == ServiceMethodOAuthValue && r.Code == 0 && c.Player != nil {
		if r.Cache, err = c.Player.Submit(); err == nil {
			r.Dirty = c.Player.Dirty.Pull()
		} else {
			r = Error(err)
		}

	}

	return r
}

// limits 检查并 lock AND reset
func verify(c *Context, handle func() error) (err error) {
	path := c.ServiceMethod()
	if strings.HasPrefix(path, ServiceMethodDebug) && !cosgo.Debug() {
		return values.Errorf(0, "unauthorized")
	}
	if !options.HasServiceMethod(path) {
		return handle() //内网通信启用玩家数据
	} else {
		//c.Binder = binder.New(binder.MIMEPROTOBUF) //外部通信
	}

	l := MethodGrade(path)
	if l == options.OAuthTypeNone {
		return handle()
	}
	if l == options.OAuthTypeOAuth {
		if guid := c.GetMetadata(options.ServiceMetadataGUID); guid == "" {
			return values.Errorf(0, "guid empty")
		} else {
			return handle()
		}
	}
	uid := c.Uid()
	if uid == 0 {
		return values.Errorf(0, "not select role")
	}

	err = players.Try(uid, func(p *player.Player) error {
		c.Player = p
		c.Player.KeepAlive(c.Unix())

		if c.Player.Lively < times.Daily(0).Unix() && c.ServiceMethod() != ServiceMethodRoleRenewal {
			return errors.ErrNeedResetSession
		}
		//尝试重新上线
		meta := c.Metadata()
		if c.Player.Status != player.StatusConnected {
			if e := players.Connect(p, meta); e != nil {
				return e
			}
		} else if session := meta[options.ServicePlayerSession]; session != p.Session {
			//return errors.ErrReplaced
		}
		//不进入用户协议 不执行submit
		c.SetValue(ServiceMethodOAuthName, ServiceMethodOAuthValue)
		return handle()
	})
	return
}
