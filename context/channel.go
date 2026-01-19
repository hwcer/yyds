package context

import (
	"encoding/json"
	"strings"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/logger"
)

// Channel 频道操作器
func (this *Context) Channel() *Channel {
	return &Channel{Context: this}
}

type Channel struct {
	*Context
}

func (this *Channel) Name(name, value string) string {
	roomArr := []string{name, value}
	roomByte, _ := json.Marshal(&roomArr)
	return string(roomByte)
}

// Join 加入频道
func (this *Channel) Join(name, value string) {
	s := strings.Join([]string{gwcfg.ServicePlayerChannelJoin, name}, "")
	this.SetMetadata(s, value)
}

// Leave  退出频道
func (this *Channel) Leave(name, value string) {
	s := strings.Join([]string{gwcfg.ServicePlayerChannelLeave, name}, "")
	this.SetMetadata(s, value)
}

// Delete 删除频道，如果消息不为空，先广播后删除
func (this *Channel) Delete(name, value string, path string, args any, req values.Metadata) {
	this.broadcast(gwcfg.MessageChannelDelete, name, value, path, args, req)
}

// Broadcast  频道广播
func (this *Channel) Broadcast(name, value string, path string, args any, req values.Metadata) {
	this.broadcast(gwcfg.MessageChannelBroadcast, name, value, path, args, req)
}

func (this *Channel) broadcast(sp string, name, value string, path string, args any, req values.Metadata) {
	if req == nil {
		req = values.Metadata{}
	}

	if _, ok := req[binder.HeaderContentType]; !ok {
		req[binder.HeaderContentType] = binder.Json.String()
	}
	req[gwcfg.ServiceMessagePath] = path
	req[gwcfg.ServiceMessageChannel] = this.Name(name, value)
	if err := client.CallWithMetadata(req, nil, gwcfg.ServiceName, sp, args, nil); err != nil {
		logger.Debug("频道广播失败:%v", err)
	}
}
