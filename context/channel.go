package context

import (
	"encoding/json"
	"strings"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/options"
)

// Channel 频道操作器
func (this *Context) Channel() *Channel {
	return &Channel{Context: this}
}

type Channel struct {
	*Context
}

//func (this *Channel) Name(name, value string) string {
//	return strings.Join([]string{name, value}, ".")
//}

// Join 加入频道
func (this *Channel) Join(name, value string) {
	s := strings.Join([]string{options.ServicePlayerRoomJoin, name}, "")
	this.SetMetadata(s, value)
}

// Leave  退出频道
func (this *Channel) Leave(name, value string) {
	s := strings.Join([]string{options.ServicePlayerRoomLeave, name}, "")
	this.SetMetadata(s, value)
}

// Broadcast  频道广播
func (this *Channel) Broadcast(path string, args any, name, value string, req values.Metadata) {
	if req == nil {
		req = values.Metadata{}
	}
	
	if _, ok := req[binder.HeaderContentType]; !ok {
		req[binder.HeaderContentType] = binder.Json.String()
	}
	req[options.ServiceMessagePath] = path
	roomArr := []string{name, value}
	roomByte, _ := json.Marshal(&roomArr)
	req[options.ServiceMessageRoom] = string(roomByte)
	if err := client.CallWithMetadata(req, nil, options.ServiceTypeGate, "channel/broadcast", args, nil); err != nil {
		logger.Debug("频道广播失败:%v", err)
	}
}
