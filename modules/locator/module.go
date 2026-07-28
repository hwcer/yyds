package locator

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/yyds/modules/locator/handle"
	"github.com/hwcer/yyds/modules/locator/master"
	"github.com/hwcer/yyds/modules/locator/model"
	"github.com/hwcer/yyds/options"
)

var m = &Module{}

var _ = handle.Register

func New() *Module {
	return m
}

type Module struct {
}

func (this *Module) Id() string {
	return options.ServiceTypeLocator
}
func (this *Module) Init() (err error) {
	if err = options.Initialize(); err != nil {
		return
	}
	if err = cosgo.Config.UnmarshalKey(options.ServiceTypeLocator, &model.Options); err != nil {
		return
	}

	return utils.Assert(model.Start)
}
func (this *Module) Start() (err error) {
	if err = master.Start(); err != nil {
		return
	}
	return subscribe()
}
func (this *Module) Close() (err error) {
	//必须关掉:pubsub 客户端是无限重连的,不关会一直重连、进程退不掉
	if emitter != nil {
		return emitter.Close()
	}
	return nil
}
