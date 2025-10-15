package options

import (
	"context"
	"net/url"
	"strconv"

	"github.com/hwcer/cosrpc"
	"github.com/smallnest/rpcx/share"
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
	sid     string //服务器
	index   uint64
	Address string //tcp@127.0.0.1:8000
	Average int    //负载
}

type Selector struct {
	all         map[string]*selectorNode
	services    map[string][]*selectorNode
	servicePath string
}

func (this *Selector) SelectWithServerId(list []*selectorNode) (r string) {
	var s *selectorNode
	for _, v := range list {
		if s == nil || v.Average < s.Average {
			s = v
		}
	}
	if s != nil {
		s.Average += 1
		r = s.Address
	}
	return
}

// Select 默认按负载
func (this *Selector) Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) (r string) {
	metadata, _ := ctx.Value(share.ReqMetaDataKey).(map[string]string)
	if metadata != nil {
		if address, ok := metadata[SelectorAddress]; ok {
			return cosrpc.AddressFormat(address)
		}
		if v, ok := metadata[SelectorServerId]; ok {
			return this.SelectWithServerId(this.services[v])
		}
	}

	var s *selectorNode
	for _, v := range this.all {
		if s == nil || v.Average < s.Average {
			s = v
		}
	}
	if s != nil {
		s.Average += 1
		r = s.Address
	}
	return
}

func (this *Selector) UpdateServer(servers map[string]string) {
	all := make(map[string]*selectorNode)
	service := make(map[string][]*selectorNode)

	//logger.Debug("===================UpdateServer:%v============================", this.servicePath)
	for address, value := range servers {
		s := &selectorNode{}
		s.Address = address
		if v, ok := this.all[address]; ok {
			s.index = v.index
		}
		if query, err := url.ParseQuery(value); err == nil {
			s.sid = query.Get(SelectorServerId)
			s.Average, _ = strconv.Atoi(query.Get(SelectorAverage))
		}
		all[address] = s
	}
	for _, v := range all {
		if v.sid != "" {
			service[v.sid] = append(service[v.sid], v)
		}
	}
	this.all = all
	this.services = service
}
