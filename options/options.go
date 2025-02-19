package options

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/utils"
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

func Initialize() error {
	if !atomic.CompareAndSwapInt32(&initialize, 0, 1) {
		return nil
	}
	if err := cosgo.Config.Unmarshal(Options); err != nil {
		return err
	}
	xshare.Options.BasePath = Options.Appid
	if Game.Time != "" {
		if t, err := times.Parse(Game.Time); err != nil {
			return err
		} else if t != nil {
			Game.timeUnix = t.Unix()
		}
	}
	if len(xshare.Service) > 0 {
		cosgo.On(cosgo.EventTypLoaded, rpcStart)
		cosgo.On(cosgo.EventTypClosing, xclient.Close)
		cosgo.On(cosgo.EventTypStopped, xserver.Close)
	}
	return nil
}

func rpcStart() error {
	return utils.Assert(xserver.Start, xclient.Start)
}

var Options = &struct {
	Data    string //静态数据地址
	Debug   bool
	Appid   string
	Master  string
	Secret  string //秘钥,必须8位
	Verify  int8   `json:"monitor"` //平台验证方式,0-不验证，1-仅仅验证签名，2-严格模式
	Service map[string]string
	Game    *game
	Gate    *gate
	Rpcx    *xshare.Rpcx
}{
	Verify:  1,
	Service: xshare.Service,
	Game:    Game,
	Gate:    Gate,
	Rpcx:    xshare.Options,
}
