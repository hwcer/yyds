package options

import "strings"

type protocol int8

const (
	ProtocolTypeWSS  int8 = 1 << 0
	ProtocolTypeTCP  int8 = 1 << 1
	ProtocolTypeHTTP int8 = 1 << 2
)

func (p protocol) Has(t int8) bool {
	v := int8(p)
	return v|t == v
}

// CMux 是否启动cmux模块
func (p protocol) CMux() bool {
	var v int8
	if p.Has(ProtocolTypeTCP) {
		v++
	}
	if p.Has(ProtocolTypeWSS) {
		v++
	}
	if p.Has(ProtocolTypeHTTP) {
		v++
	}
	return v > 1
}

var Gate = &gate{
	Login:     "/game/login",
	Prefix:    "handle",
	Address:   "0.0.0.0:80",
	Protocol:  2,
	Websocket: "ws",
}

type gate = struct {
	Login    string   `json:"login"`    //登录接口
	Prefix   string   `json:"prefix"`   //路由强制前缀
	Address  string   `json:"address"`  //连接地址
	Protocol protocol `json:"protocol"` //1-短链接，2-长连接，3-长短链接全开
	//Broadcast int8     `json:"broadcast"` //Push message 0-关闭，1-双向通信，2-独立启动服务器,推送消息必须启用长链接
	Websocket string `json:"websocket"` //开启websocket时,路由前缀
	//WSVerify  bool   `json:"WSVerify"`
}

func GetServiceSelectorAddress(k string) string {
	return ServicePlayerSelector + strings.ToLower(k)
}