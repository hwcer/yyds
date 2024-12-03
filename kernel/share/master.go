package share

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosgo/request"
	"github.com/hwcer/cosgo/values"
	"strings"
	"sync"
)

type MasterApiType string

const (
	//MasterApiTypeServiceStart     MasterApiType = "/service/start"
	MasterApiTypeGameServerUpdate = "/service/update"
	MasterApiTypeServiceClose     = "/service/close"
	//MasterApiTypeUCenterStart               = "/ucenter/start" //ucenter启动
	MasterApiTypeOrderCreate  = "/order/create"
	MasterApiTypeOrderRefresh = "/order/refresh" //重新拉起之前放弃的订单
	MasterApiTypeOrderSubmit  = "/order/submit"
	MasterApiTypeConfigInfo   = "/config/info"

	MasterApiTypeConfigCreate = "/config/create"
	MasterApiTypeAccessUpdate = "/access/update"
)

var Master = &master{}

func init() {
	Master.client = request.New()
}

type master struct {
	//url  string
	auth   *request.OAuth
	client *request.Client
	mutex  sync.Mutex
}

func (m *master) init() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.auth == nil && options.Options.Verify > 0 {
		m.auth = request.NewOAuth(options.Options.Appid, options.Options.Secret)
		if options.Options.Verify >= 2 {
			m.auth.Strict = true
		}
		m.client.Use(m.auth.Request)
	}
}

func (m *master) OAuth() *request.OAuth {
	if m.auth == nil {
		m.init()
	}
	return m.auth
}

func (m *master) Post(api MasterApiType, args interface{}, reply interface{}) (err error) {
	if options.Options.Master == "" {
		return ErrMasterEmpty
	}
	url := options.Options.Master
	if strings.HasSuffix(url, "/") {
		url = strings.TrimSuffix(url, "/")
	}
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}

	b := &strings.Builder{}
	b.WriteString(url)
	b.WriteString("/")
	b.WriteString(options.Options.Appid)
	b.WriteString(string(api))
	msg := values.Parse(nil)
	_ = m.OAuth()

	if err = m.client.Post(b.String(), args, msg); err != nil {
		logger.Trace("加载master错误:%v", b.String())
		return
	}
	if msg.Code != 0 {
		return msg
	}
	if reply != nil {
		err = msg.Unmarshal(reply)
	}
	return
}
