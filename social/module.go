package social

import (
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/social/handle"
	"github.com/hwcer/yyds/social/master"
	"github.com/hwcer/yyds/social/model"
)

var m = &Module{}

var _ = handle.Register

func New() *Module {
	return m
}

type Module struct {
}

func (this *Module) Id() string {
	return options.ServiceTypeSocial
}
func (this *Module) Init() (err error) {
	if err = options.Initialize(); err != nil {
		return
	}
	if err = cosgo.Config.UnmarshalKey(options.ServiceTypeSocial, &model.Options); err != nil {
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
