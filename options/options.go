package options

import (
	"sync/atomic"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosrpc/redis"
	"github.com/hwcer/gateway/gwcfg"
)

var initialize atomic.Bool

// Deprecated: 用 gwcfg.ServiceTypeXxx。
//
// 微服务名已挪到 gwcfg：网关要按服务名做路由与转发（如心跳补服务名前缀），
// 却不能反向依赖 yyds——那会形成 gateway ← yyds 的循环。这里保留同名常量只为
// 兼容既有引用，新代码一律写 gwcfg.ServiceTypeXxx。
const (
	ServiceTypeGate    = gwcfg.ServiceTypeGate
	ServiceTypeGame    = gwcfg.ServiceTypeGame
	ServiceTypeWorld   = gwcfg.ServiceTypeWorld
	ServiceTypeBattle  = gwcfg.ServiceTypeBattle
	ServiceTypeRooms   = gwcfg.ServiceTypeRooms
	ServiceTypeSocial  = gwcfg.ServiceTypeSocial
	ServiceTypeLocator = gwcfg.ServiceTypeLocator
)

func init() {
	cosgo.On(cosgo.EventTypReload, reload)
}

// reload 重新加载配置
// 只能是业务层面参数生效，Debug,Developer,Maintenance
// 无法重启服务(rpc,web server)
func reload() error {
	return cosgo.Config.Unmarshal(Options)
}

func Initialize() (err error) {
	if !initialize.CompareAndSwap(false, true) {
		return nil
	}
	if err = reload(); err != nil {
		return err
	}
	//启动 Redis 服务发现
	if err = redis.Start(); err != nil {
		return err
	}

	return nil
}

var Options = &struct {
	Data   string //静态数据地址
	Debug  bool
	Appid  string //appid
	Master string //游戏中控地址
	Secret string `json:"secret"` //秘钥,必须8位
	Verify int8   `json:"verify"` //平台验证方式,0-不验证，1-仅仅验证签名，2-严格模式
	Binder string `json:"binder"` //公网请求默认序列化方式，默认JSON
	Game   *game  `json:"game"`
}{
	Verify: 1,
	Binder: "json",
	Game:   Game,
}

func GetServerTime() int64 {
	return Game.Unix
}
