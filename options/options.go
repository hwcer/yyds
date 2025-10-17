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

func Initialize() (err error) {
	if !atomic.CompareAndSwapInt32(&initialize, 0, 1) {
		return nil
	}
	if err = cosgo.Config.Unmarshal(Options); err != nil {
		return err
	}
	cosrpc.SetBasePath(Options.Appid)
	cosrpc.Selector.Set(ServiceTypeGate, NewSelector(ServiceTypeGate))
	cosrpc.Selector.Set(ServiceTypeGame, NewSelector(ServiceTypeGame))

	if r := server.GetRegistry(); r.Len() > 0 {
		var addr string
		var register server.Register
		if addr, err = rpcxRedisAddress(); err == nil && addr != "" {
			register, err = Register(cosrpc.Address())
		}
		if err != nil {
			return err
		}
		server.SetRegister(register)
	}
	if len(cosrpc.Service) > 0 {
		client.SetDiscovery(Discovery)
	}

	//if Options.TimeReset != 0 {
	//	times.SetTimeReset(Options.TimeReset)
	//}
	return nil
}

var Options = &struct {
	Data      string //静态数据地址
	Debug     bool
	Appid     string
	Master    string
	Secret    string            `json:"secret"`    //秘钥,必须8位
	Verify    int8              `json:"verify"`    //平台验证方式,0-不验证，1-仅仅验证签名，2-严格模式
	Binder    string            `json:"binder"`    //公网请求默认序列化方式，默认JSON
	Service   map[string]string `json:"service"`   //
	Developer string            `json:"developer"` //超级用户秘钥，可以使用账号直接登录,开启游戏内一些功能
	//Superuser string            `json:"superuser"` //超级用户秘钥,开启游戏内一些功能
	//TimeReset int64             `json:"TimeReset"` //每日几点重置时间
	Game *game           `json:"game"`
	Gate *gate           `json:"gate"`
	Rpcx *cosrpc.Options `json:"rpcx"`
}{
	Verify:  1,
	Binder:  "json",
	Service: cosrpc.Service,
	Game:    Game,
	Gate:    Gate,
	Rpcx:    cosrpc.Config,
}

// Cookies 仅仅 http+json模式下 Cookie模板,网关会将 %CookieKey% %CookieValue% 替换成对应值
//var Cookies = &struct {
//	Name  string
//	Value string
//}{
//	Name:  "%CookieKey%",
//	Value: "%CookieValue%",
//}
