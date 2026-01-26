package options

import (
	"sync/atomic"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosrpc/redis"
)

var initialize atomic.Bool

const (
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
