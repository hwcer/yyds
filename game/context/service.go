package context

import (
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/xserver"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/game/players/player"
	"reflect"
	"runtime/debug"
	"strconv"
)

/*
所有接口都必须已经登录
使用updater时必须使用playerHandle.data()来获取updater
*/

var Service = xserver.Service(options.ServiceTypeGame, handlerMetadata, handlerCaller, handlerFilter)

func Register(i interface{}, prefix ...string) {
	var arr []string
	if options.Gate.Prefix != "" {
		arr = append(arr, options.Gate.Prefix)
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

type Caller interface {
	Caller(node *registry.Node, c *Context) interface{}
}

var handlerFilter xshare.HandlerFilter = func(node *registry.Node) bool {
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
		if _, ok := node.Binder().(Caller); !ok {
			v := reflect.Indirect(reflect.ValueOf(node.Binder()))
			logger.Debug("[%v]未正确实现Caller方法,会影响程序性能", v.Type().String())
		}
		return true
	}
}

var handlerCaller xshare.HandlerCaller = func(node *registry.Node, sc *xshare.Context) (reply any, err error) {
	c := &Context{Context: sc}
	defer func() {
		if v := recover(); v != nil {
			if cosgo.Debug() {
				reply = Errorf(500, v)
			} else {
				reply = Errorf(500, "server recover error")
			}
			logger.Trace("server recover error:%v\n%v", v, string(debug.Stack()))
		}
	}()
	ex := verify(c, func() error {
		c.reset()
		defer c.release()
		//判定重发
		if rid := getMetadataRequestId(c.Context); rid > 0 {
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
		r := caller(c, node)
		reply, err = serialize(sc, r)
		return nil
	})
	if ex != nil {
		return serialize(sc, values.Parse(ex))
	}
	return
}

var handlerMetadata xshare.HandlerMetadata = func() string {
	return fmt.Sprintf("%v=%v", options.Options.Appid, options.Game.Sid)
}

func serialize(c *xshare.Context, reply interface{}) ([]byte, error) {
	switch v := reply.(type) {
	case []byte:
		return v, nil
	case *Message:
		if v.Code == 0 && v.Data == nil {
			return []byte{}, nil //长连接返回 nil 不自动回复
		} else {
			return xshare.Binder.Marshal(reply)
		}
	default:
		return xshare.Binder.Marshal(reply)
	}
}

func getMetadataRequestId(sc *xshare.Context) int32 {
	rid := sc.GetMetadata(options.ServiceMetadataRequestId)
	if rid == "" {
		return 0
	}
	v, _ := strconv.Atoi(rid)
	return int32(v)
}
