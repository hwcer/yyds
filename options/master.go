package options

import (
	"strings"

	"github.com/hwcer/cosgo/await"
	"github.com/hwcer/cosgo/request"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/errors"
)

type MasterApiType string

const (
	MasterApiTypeGameServerStart = "/server/start"
	MasterApiTypeGameServerClose = "/server/close"
	MasterApiTypeGameServerInfo  = "/server/info" //查询本服状态,收到 master 变更事件后回拉
	MasterApiTypeOrderCreate     = "/order/create"
	MasterApiTypeOrderRefresh    = "/order/refresh" //重新拉起之前放弃的订单
	MasterApiTypeOrderRestore    = "/order/restore"
	MasterApiTypeOrderSubmit     = "/order/submit"
	MasterApiTypeConfigInfo      = "/config/info"

	MasterApiTypeConfigCreate = "/config/create"
	MasterApiTypeAccessUpdate = "/access/update"
)

var Master = &master{}

func init() {
	Master.client = request.New()
	Master.initialize = await.NewInitialize()
}

type master struct {
	client     *request.Client
	started    bool
	initialize *await.Initialize
}

func (m *master) init() error {
	if Options.Verify > 0 {
		var strict bool
		if Options.Verify >= 2 {
			strict = true
		}
		m.client.OAuth(Options.Appid, Options.Secret, strict)
	}
	m.started = true
	return nil
}

func (m *master) Post(api MasterApiType, args interface{}, reply interface{}) (err error) {
	if Options.Master == "" {
		return errors.ErrMasterEmpty
	}
	if !m.started {
		_ = m.initialize.Try(m.init)
	}

	url := Options.Master
	if strings.HasSuffix(url, "/") {
		url = strings.TrimSuffix(url, "/")
	}
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}

	b := &strings.Builder{}
	b.WriteString(url)
	b.WriteString("/")
	b.WriteString(Options.Appid)
	b.WriteString(string(api))
	msg := &values.Message{}

	if err = m.client.Post(b.String(), args, msg); err != nil {
		logger.Trace("加载master错误:%v", b.String())
		return
	}
	if msg.Code != 0 {
		return msg
	}
	//reply 是 *values.Message 时直接整体交出去,不在这一层解析 Data。
	//给的是调用方自行决定怎么解的余地:master 各接口回包格式并不统一
	//(同一个接口老版本回 true、新版本回对象),塞进固定结构会解析失败,
	//而 Data 此时是 json.RawMessage,调用方拿到后再 Unmarshal 到自己的类型即可
	if v, ok := reply.(*values.Message); ok {
		*v = *msg
		return
	}
	if reply != nil {
		err = msg.Unmarshal(reply)
	}
	return
}
