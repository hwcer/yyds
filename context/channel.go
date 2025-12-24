package context

import (
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

func (this *Channel) Name(name, value string) string {
	return strings.Join([]string{name, value}, ".")
}

// Join 加入频道
func (this *Channel) Join(name, value string) {
	s := this.Name(name, value)
	this.SetMetadata(options.ServicePlayerRoomJoin, s)
}

// Leave  退出频道
func (this *Channel) Leave(name, value string) {
	s := this.Name(name, value)
	this.SetMetadata(options.ServicePlayerRoomLeave, s)
}

// Broadcast  频道广播
func (this *Channel) Broadcast(path string, args any, name, value string) {
	req := values.Metadata{}
	req[binder.HeaderContentType] = binder.Protobuf.String()
	req[options.ServiceMessagePath] = path
	req[options.ServiceMessageRoom] = this.Name(name, value)
	if err := client.CallWithMetadata(req, nil, options.ServiceTypeGate, "broadcast", args, nil); err != nil {
		logger.Error(err)
	}
}
