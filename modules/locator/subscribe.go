package locator

import (
	"encoding/json"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/logger"
	"github.com/hwcer/pubsub"
	pscosnet "github.com/hwcer/pubsub/cosnet"
	"github.com/hwcer/yyds/modules/locator/model"
	"github.com/hwcer/yyds/options"
)

/*
订阅 master 的事件总线，按事件里的转发范围用 rpcx 转发给游戏服。

事件本身只带路由信息不带业务数据，游戏服收到后自己回 master 拉最新值，
所以这里不关心事件内容，只负责决定"发给谁"。
*/

// topic 名与 master 侧 share.TopicXxx 对齐，改动需两边同步。
// master 可以接入多款游戏，下发的事件按 appid 分区（"{topic}/{appid}"），
// 这里只订本 appid 那一档，收不到别家游戏的事件。
const (
	topicServer = "server"
	topicConfig = "config"
)

// topic 拼出本游戏的分区名，与 master 侧 share.Topic 保持一致
func topic(name string) string {
	if options.Options.Appid == "" {
		return name
	}
	return name + "/" + options.Options.Appid
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

// 事件转发队列。
//
// pubsub 的订阅回调是在 socket 的读协程里同步执行的
// （readMsg → handle → clientHandler.Message → deliverLocal → handler），
// 直接在回调里做 rpc 会把读循环卡住：心跳回包读不到，连接会被误判为掉线，
// 后续事件也全堵在 TCP 缓冲里。所以回调只入队，投递交给单独的协程。
//
// 单消费者而不是每事件一个协程：游戏服收到信号后是回 master 拉最新值，
// 并发投递会让两个事件的回拉乱序、旧值可能覆盖新值。
var events = make(chan *event, 256)

type event struct {
	path string
	body []byte
	msg  *message
}

func dispatcher() {
	for e := range events {
		switch e.msg.Scope {
		case scopeEnable:
			for _, sid := range e.msg.GZone {
				send(sid, e.path, e.body)
			}
		case scopeAll, scopeIgnore:
			//屏蔽名单只能由游戏服自己判断:locator 不掌握全部区服列表,
			//统一广播并把 GZone 原样带过去,游戏服按 Scope 自行过滤。
			broadcast(e.path, e.body)
		}
	}
}

// subscribe 启动对 master 事件总线的订阅
func subscribe() error {
	if model.Options.Pubsub == "" {
		return nil
	}
	tr := pscosnet.Connect(model.Options.Pubsub)
	//默认重连 10 次(约 3 分钟)后彻底放弃、表现为永久静默失联，必须无限重连；
	//退避默认封顶 30 秒，master 重启后最坏空档 30 秒，期间发布的事件全丢。
	//这条连接平时零流量、重连成本极低，压到 0.5 秒起步、最多 3 秒。
	opt := tr.Options()
	opt.ClientReconnectMax = 0
	opt.ClientReconnectTime = 500
	opt.ClientReconnectMaxDelay = 3000

	emitter = pubsub.New()
	//顺序不能反:Subscribe 只在已注册 transport 时才会把订阅同步到服务端
	emitter.Use(tr)
	emitter.Subscribe(topic(topicServer), forward(pathServerUpdate))
	emitter.Subscribe(topic(topicConfig), forward(pathConfigUpdate))
	go dispatcher()

	//Connect 失败会一直重试并阻塞,不能拖住 locator 启动
	go func() {
		if err := emitter.Start(); err != nil {
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
		//只入队，绝不在这里做 rpc：本回调跑在 socket 读协程上
		select {
		case events <- &event{path: path, body: body, msg: msg}:
		default:
			logger.Alert("master 事件队列已满,丢弃:topic=%v,id=%v", e.Topic, msg.Id)
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
