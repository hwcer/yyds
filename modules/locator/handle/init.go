package handle

import (
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/cosrpc/server"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/modules/locator/model"
)

var db = model.DB()
var Service = server.Service(gwcfg.ServiceTypeLocator)

// Register 注册一个 RPC 接口。
//
// 🔴 **本包的接口只对内，网关够不着。**
//
// 名字叫 handle 容易和游戏服那个 handle 搞混,但两者语义相反:
//
//	yyds/context.Register   游戏服的 handle —— 会加上网关 Prefix,是【玩家可达】的外网接口,
//	                        请求要过鉴权分级、要加载玩家数据
//	本函数                   locator 的 handle —— 不加任何 Prefix,只有【服务间 rpcx】调得到,
//	                        没有鉴权、没有玩家上下文
//
// 差别来自 Service 的建法:上面那个是 server.Service(name, handlerCaller, handlerFilter)
// (带 yyds 的接入层),这里是裸的 server.Service(name)。
//
// 所以在本包加接口时不必考虑鉴权与玩家上下文,但也【不要】假设调用方是可信的业务层就省掉
// 参数校验 —— 服务间调用同样会传错。
//
// 对外的 HTTP 入口在 master/ 包(cosweb + CORS,把请求代理到游戏服),与本包不是一回事。
func Register(i interface{}, prefix ...string) {
	var arr []string
	if len(prefix) > 0 {
		arr = append(arr, prefix...)
	} else {
		arr = append(arr, "%v")
	}
	if err := Service.Register(i, arr...); err != nil {
		logger.Fatal("%v", err)
	}
}
