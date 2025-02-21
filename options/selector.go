package options

import (
	"context"
	"fmt"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/smallnest/rpcx/share"
	"net/url"
	"strconv"
	"strings"
)

const (
	SelectorAverage  = "_rpc_srv_avg"
	SelectorAddress  = "_rpc_srv_addr" //rpc服务器ID,selector 中固定转发地址
	SelectorServerId = "_rpc_srv_sid"  //服务器编号
)

func NewSelector(servicePath string) *Selector {
	return &Selector{servicePath: servicePath}
}

type selectorNode struct {
	Address string //tcp@127.0.0.1:8000
	Average int    //负载
	//ServerId string //服务器
}

type Selector struct {
	share       []*selectorNode
	services    map[string][]*selectorNode
	servicePath string
}

// Select 默认按负载
func (this *Selector) Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) string {
	metadata, _ := ctx.Value(share.ReqMetaDataKey).(map[string]string)
	var list []*selectorNode
	if metadata != nil {
		if address, ok := metadata[SelectorAddress]; ok {
			return xshare.AddressFormat(address)
		}
		if v, ok := metadata[SelectorServerId]; ok {
			list = this.services[v]
		} else {
			list = this.share
		}
	} else {
		list = this.share
	}
	var s *selectorNode
	for _, v := range list {
		if s == nil || v.Average < s.Average {
			s = v
		}
	}
	if s == nil {
		return ""
	}
	s.Average += 1
	return s.Address
}

func (this *Selector) UpdateServer(servers map[string]string) {
	var nodes []*selectorNode
	service := make(map[string][]*selectorNode)

	//logger.Debug("===================UpdateServer:%v============================", this.servicePath)
	prefix := fmt.Sprintf("%v/%v/", xshare.Options.BasePath, this.servicePath)
	for address, value := range servers {
		if !strings.HasPrefix(address, prefix) {
			continue
		}
		//logger.Debug("UpdateServer  address：%v value:%v", address, value)
		s := &selectorNode{}
		s.Address = strings.TrimPrefix(address, prefix)
		if query, err := url.ParseQuery(value); err == nil {
			s.Average, _ = strconv.Atoi(query.Get(SelectorAverage))
			if sid := query.Get(SelectorServerId); sid != "" {
				service[sid] = append(service[sid], s)
			} else {
				nodes = append(nodes, s)
			}
		}
	}
	this.share = nodes
	this.services = service
}
