package locator

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/yyds/locator/handle"
	"github.com/hwcer/yyds/locator/master"
	"github.com/hwcer/yyds/locator/model"
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
	return master.Start()
}
func (this *Module) Close() (err error) {
	return nil
}
