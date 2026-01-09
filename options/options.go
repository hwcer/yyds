package options

import (
	"sync/atomic"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosrpc"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/cosrpc/server"
)

var initialize int32

const (
	ServiceTypeGate    = "gate"
	ServiceTypeGame    = "game"
	ServiceTypeWorld   = "world"   //世界服
	ServiceTypeBattle  = "battle"  //战斗服
	ServiceTypeRooms   = "rooms"   //游戏大厅
	ServiceTypeSocial  = "social"  //社交用户中心
	ServiceTypeLocator = "locator" //角色定位中心
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
	if !atomic.CompareAndSwapInt32(&initialize, 0, 1) {
		return nil
	}
	if err = reload(); err != nil {
		return err
	}
	cosrpc.Selector.Set(ServiceTypeGate, NewSelector(ServiceTypeGate))
	cosrpc.Selector.Set(ServiceTypeGame, NewSelector(ServiceTypeGame))

	if Options.Rpcx.Redis != "" {
		server.SetRegister(Register)
		client.SetDiscovery(Discovery)
	}

	return nil
}

type Rpcx struct {
	*cosrpc.Options `json:",inline" mapstructure:",squash"`
	Redis           string `json:"redis" mapstructure:"redis"`
}

var Options = &struct {
	Data        string //静态数据地址
	Debug       bool
	Appid       string
	Master      string
	Secret      string            `json:"secret"`      //秘钥,必须8位
	Verify      int8              `json:"verify"`      //平台验证方式,0-不验证，1-仅仅验证签名，2-严格模式
	Binder      string            `json:"binder"`      //公网请求默认序列化方式，默认JSON
	Service     map[string]string `json:"service"`     //
	Developer   string            `json:"developer"`   //超级用户秘钥，可以使用账号直接登录,开启游戏内一些功能
	Maintenance bool              `json:"maintenance"` //进入维护模式，仅仅开发人员允许进入
	Game        *game             `json:"game"`
	Gate        *gate             `json:"gate"`
	Rpcx        *Rpcx             `json:"rpcx"`
}{
	Verify:  1,
	Binder:  "json",
	Service: cosrpc.Service,
	Game:    Game,
	Gate:    Gate,
	Rpcx:    &Rpcx{Options: cosrpc.Config},
}

// Cookies 仅仅 http+json模式下 Cookie模板,网关会将 %CookieKey% %CookieValue% 替换成对应值
//var Cookies = &struct {
//	Name  string
//	Value string
//}{
//	Name:  "%CookieKey%",
//	Value: "%CookieValue%",
//}
