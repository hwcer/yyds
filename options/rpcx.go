package options

import (
	"errors"
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosrpc"
	"github.com/hwcer/cosrpc/redis"
	"github.com/rpcxio/libkv/store"
	"github.com/smallnest/rpcx/client"
	"net/url"
	"time"
)

//var Rpcx = &rpcx{
//	Rpcx: xshare.Options,
//}
//
//type rpcx struct {
//	*xshare.Rpcx
//	Redis string `json:"redis"` //rpc服务器注册发现 pub/sub 订阅服务
//}

//var rpcxRegister *redis.Register

func Discovery(servicePath string) (client.ServiceDiscovery, error) {
	address, opt, err := rpcxRedisParse()
	if err != nil {
		return nil, err
	}
	var discovery *redis.Discovery
	discovery, err = redis.NewDiscovery(cosrpc.Config.BasePath, servicePath, address, opt)
	if err != nil {
		return nil, err
	}
	return discovery, nil
}

func Register(urlRpcxAddr *utils.Address) (*redis.Register, error) {
	address, opt, err := rpcxRedisParse()
	if err != nil {
		return nil, err
	}
	host := urlRpcxAddr.Host
	if utils.LocalValid(host) {
		host, err = utils.LocalIPv4()
	}
	if err != nil {
		return nil, err
	}
	rpcxRegister := &redis.Register{
		ServiceAddress: fmt.Sprintf("%v%v:%v", cosrpc.AddressPrefix(), host, urlRpcxAddr.Port),
		RedisServers:   address,
		BasePath:       cosrpc.Config.BasePath,
		Options:        opt,
		UpdateInterval: time.Second * 10,
	}
	return rpcxRegister, nil
}

func rpcxRedisAddress() (addr string, err error) {
	err = cosgo.Config.UnmarshalKey("rpcx.redis", &addr)
	return
}

func rpcxRedisParse() (address []string, opts *store.Config, err error) {
	var addr string
	if addr, err = rpcxRedisAddress(); err != nil {
		return
	} else if addr == "" {
		return nil, nil, errors.New("redis address is empty")
	}
	var uri *url.URL
	uri, err = utils.NewUrl(addr, "tcp")
	if err != nil {
		return
	}
	address = []string{uri.Host}
	opts = &store.Config{}
	query := uri.Query()
	opts.Password = query.Get("password")
	if query.Has("db") {
		opts.Bucket = query.Get("db")
	} else {
		opts.Bucket = "13"
	}
	return
}
