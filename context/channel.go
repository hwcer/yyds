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

func (this *Channel) Name(s ...string) string {
	return strings.Join(s, ".")
}

// Join 加入频道
func (this *Channel) Join(name ...string) {
	this.SetMetadata(options.ServicePlayerRoomJoin, this.Name(name...))
}

// Leave  退出频道
func (this *Channel) Leave(name ...string) {
	this.SetMetadata(options.ServicePlayerRoomLeave, this.Name(name...))
}

// Broadcast  频道广播
func (this *Channel) Broadcast(path string, args any, name ...string) {
	req := values.Metadata{}
	req[binder.HeaderContentType] = binder.Protobuf.String()
	req[options.ServiceMessagePath] = path
	req[options.ServiceMessageRoom] = this.Name(name...)
	if err := client.CallWithMetadata(req, nil, options.ServiceTypeGate, "broadcast", args, nil); err != nil {
		logger.Error(err)
	}
}
