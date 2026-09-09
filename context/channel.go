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

// Kick 踢出频道中的指定玩家(如会长踢人),uid 为被踢玩家的角色ID
// 与 Join/Leave 一样挂在**当前请求者**的响应 metadata 上,由网关回包时执行;
// 值编码为[频道值,被踢UID],网关经 UID->GUID 映射定位被踢者会话
func (this *Channel) Kick(name, value string, uid string) {
	s := strings.Join([]string{gwcfg.ServicePlayerChannelKick, name}, "")
	this.SetMetadata(s, this.Name(value, uid))
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
	if err := client.CallWithMetadata(req, nil, gwcfg.ServiceTypeGate, sp, args, nil); err != nil {
		logger.Debug("频道广播失败:%v", err)
	}
}
