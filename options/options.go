package options

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosrpc/xclient"
	"github.com/hwcer/cosrpc/xserver"
	"github.com/hwcer/cosrpc/xshare"
	"sync/atomic"
)

var initialize int32

const (
	ServiceTypeGate   = "gate"
	ServiceTypeGame   = "game"
	ServiceTypeChat   = "chat" //聊天
	ServiceTypeBattle = "battle"
	ServiceTypeRooms  = "rooms"  //游戏大厅
	ServiceTypeSocial = "social" //社交用户中心
)

func Initialize() (err error) {
	if !atomic.CompareAndSwapInt32(&initialize, 0, 1) {
		return nil
	}
	if err = cosgo.Config.Unmarshal(Options); err != nil {
		return err
	}
	xshare.Selector.Set(ServiceTypeGate, NewSelector(ServiceTypeGate))
	xshare.Selector.Set(ServiceTypeGame, NewSelector(ServiceTypeGame))
	xshare.Options.BasePath = Options.Appid

	if len(xshare.Service) > 0 {
		cosgo.On(cosgo.EventTypLoaded, rpcStart)
		cosgo.On(cosgo.EventTypStopped, xserver.Close)
	}
	if Options.TimeReset != 0 {
		times.SetTimeReset(Options.TimeReset)
	}
	return nil
}

func rpcStart() (err error) {
	var addr string
	var register xserver.Register
	if addr, err = rpcxRedisAddress(); err == nil && addr != "" {
		register, err = Register(xshare.Address())
	}
	if err != nil {
		return err
	}
	if err = xserver.Start(register); err != nil {
		return err
	}
	if err = xclient.Start(Discovery); err != nil {
		return err
	}
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
	TimeReset int64             `json:"TimeReset"` //每日几点重置时间
	Game      *game
	Gate      *gate
	Rpcx      *xshare.Rpcx
}{
	Verify:  1,
	Binder:  "json",
	Service: xshare.Service,
	Game:    Game,
	Gate:    Gate,
	Rpcx:    xshare.Options,
}

// Cookies 仅仅 http+json模式下 Cookie模板,网关会将 %CookieKey% %CookieValue% 替换成对应值
var Cookies = &struct {
	Name  string
	Value string
}{
	Name:  "%CookieKey%",
	Value: "%CookieValue%",
}
