package locator

import (
	"encoding/json"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/logger"
	"github.com/hwcer/pubsub"
	"github.com/hwcer/pubsub/transport"
	"github.com/hwcer/yyds/modules/locator/model"
	"github.com/hwcer/yyds/options"
)

/*
订阅 master 的事件总线，按事件里的转发范围用 rpcx 转发给游戏服。

事件本身只带路由信息不带业务数据，游戏服收到后自己回 master 拉最新值，
所以这里不关心事件内容，只负责决定"发给谁"。
*/

// topic 名与 master 侧 share.TopicXxx 对齐，改动需两边同步。
// master 可以接入多款游戏，下发的事件按 appid 分区（"{topic}.{appid}"），
// 这里只订本 appid 那一档，收不到别家游戏的事件。
const (
	topicServer = "server"
	topicConfig = "config"
	//分段符必须与 master 侧 share.TopicSeparator 一致：
	//pubsub 的通配 * 编译为 [^.]+，按 "." 分段
	topicSeparator = "."
)

// topic 拼出本游戏的分区名，与 master 侧 share.Topic 保持一致
func topic(name string) string {
	if options.Options.Appid == "" {
		return name
	}
	return name + topicSeparator + options.Options.Appid
}

// 游戏服侧接收事件的私有路由
const (
	pathServerUpdate = "server/update"
	pathConfigUpdate = "config/update"
)

// scope 与 master 侧 share.Scope 对齐
type scope int8

const (
	scopeNone   scope = 0 //不转发
	scopeAll    scope = 1 //不限制，转发给所有游戏服
	scopeIgnore scope = 2 //屏蔽 GZone 列表中的区服
	scopeEnable scope = 3 //只转发给 GZone 列表中的区服
)

// message 与 master 侧 share.Message 对齐
type message struct {
	Id    string  `json:"id"`
	Type  int8    `json:"type"`
	Time  int64   `json:"time"`
	Scope scope   `json:"scope"`
	GZone []int32 `json:"GZone"`
}

var emitter *pubsub.PubSub

// subscribe 启动对 master 事件总线的订阅。
// 未配置地址即不启动，与 master.Start 对 Address 的处理保持一致。
func subscribe() error {
	if model.Options.Pubsub == "" {
		logger.Alert("未配置 [locator] pubsub,不订阅 master 事件,服务器/配置变更将无法下发到游戏服")
		return nil
	}
	//地址按协议选择实现：tcp://（默认，可省略）走 cosnet，redis:// 走 redis pub/sub。
	//重连参数由工厂固化，不需要在这里调。
	tr, err := transport.Connect(model.Options.Pubsub)
	if err != nil {
		return err
	}
	//订阅回调由 pubsub 在独立协程上串行投递，不会阻塞传输层的网络读协程，
	//所以 forward 里可以放心做同步 rpc；投递不出去（队列满 / 回调 panic）时 pubsub 自己会打日志
	emitter = pubsub.New()
	//顺序不能反:Subscribe 只在已注册 transport 时才会把订阅同步到服务端
	emitter.Use(tr)
	emitter.Subscribe(topic(topicServer), forward(pathServerUpdate))
	emitter.Subscribe(topic(topicConfig), forward(pathConfigUpdate))

	//Connect 失败会一直重试并阻塞,不能拖住 locator 启动
	go func() {
		if err = emitter.Start(); err != nil {
			logger.Alert("订阅 master 事件总线失败:%v,address:%v", err, model.Options.Pubsub)
		} else {
			logger.Trace("已订阅 master 事件总线:%v,appid:%v", model.Options.Pubsub, options.Options.Appid)
		}
	}()
	return nil
}

// forward 生成事件处理器，把事件按转发范围投递到游戏服的指定路由
func forward(path string) pubsub.Handler {
	return func(e *pubsub.Event) {
		msg := &message{}
		if err := e.Unmarshal(msg); err != nil {
			logger.Alert("master 事件解析失败:topic=%v,err=%v", e.Topic, err)
			return
		}
		if msg.Scope == scopeNone {
			return //纯 master 内部事件
		}
		body, err := json.Marshal(msg)
		if err != nil {
			logger.Alert("master 事件序列化失败:topic=%v,err=%v", e.Topic, err)
			return
		}
		switch msg.Scope {
		case scopeEnable:
			for _, sid := range msg.GZone {
				send(sid, path, body)
			}
		case scopeAll, scopeIgnore:
			//屏蔽名单(scopeIgnore)统一广播,把 GZone 原样带过去由游戏服自行过滤,
			//而不是在这里排除掉黑名单里的区服:
			//  1.rpcx 的 Broadcast 直接遍历自己的 servers 建连接群发,不经过 selector,
			//    在 selector 上挂过滤函数对广播无效;
			//  2.只有 discovery 模式才有 selector 实例(见 cosrpc client.selector),
			//    local/process 模式下拿不到区服列表。
			//游戏服侧的过滤本来也必须保留(防列表过期、防投递错服),放在那边是唯一出处。
			broadcast(path, body)
		}
	}
}

func send(sid int32, path string, body []byte) {
	req := values.Metadata{}
	req[gwcfg.ServiceMetadataServerId] = values.Sprintf("%d", sid)
	req[binder.HeaderContentType] = binder.Json.Name()
	if err := client.CallWithMetadata(req, nil, options.ServiceTypeGame, path, body, nil); err != nil {
		logger.Alert("转发 master 事件失败:sid=%v,path=%v,err=%v", sid, path, err)
	}
}

func broadcast(path string, body []byte) {
	req := values.Metadata{}
	req[binder.HeaderContentType] = binder.Json.Name()
	ctx, cancel := client.WithTimeout(req, nil)
	defer cancel()
	if err := client.Broadcast(ctx, options.ServiceTypeGame, path, body, nil); err != nil {
		logger.Alert("广播 master 事件失败:path=%v,err=%v", path, err)
	}
}
