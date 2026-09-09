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

// metadata 频道命令统一编码:key = 命令前缀 + ["name","value"],频道身份由key表达;
// value 仅 Kick 使用(被踢玩家UID),Join/Leave 为空
func (this *Channel) metadata(prefix, name, value string) string {
	return strings.Join([]string{prefix, this.Name(name, value)}, "")
}

// Join 加入频道
func (this *Channel) Join(name, value string) {
	this.SetMetadata(this.metadata(gwcfg.ServicePlayerChannelJoin, name, value), "")
}

// Leave  退出频道
func (this *Channel) Leave(name, value string) {
	this.SetMetadata(this.metadata(gwcfg.ServicePlayerChannelLeave, name, value), "")
}

// Kick 踢出频道中的指定玩家(如会长踢人),uid 为被踢玩家的角色ID
// 与 Join/Leave 一样挂在**当前请求者**的响应 metadata 上,由网关回包时执行
func (this *Channel) Kick(name, value string, uid string) {
	this.SetMetadata(this.metadata(gwcfg.ServicePlayerChannelKick, name, value), uid)
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
