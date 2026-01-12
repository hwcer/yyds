package social

import (
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/yyds/modules/social/model"
)

var Graph = model.Graph

// Start 直接启用嵌入模式，不需要额外配置数据，不需要启用Module
func Start(service *registry.Service, mo *cosmo.DB, getter model.Handle) error {
	model.SetPlayers(getter)
	model.SetDatabase(mo)
	return service.Register(&Friend{})
}
