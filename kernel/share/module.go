package share

import (
	"errors"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosrpc/xserver"
)

var mod *Module

const moduleName = "share"

func init() {
	logger.SetPathTrim("src")
	logger.SetCallDepth(4)
}

func New() *Module {
	if mod == nil {
		mod = &Module{}
	}
	return mod
}

type Module struct {
	cosgo.Module
}

func (this *Module) Id() string {
	return moduleName
}
func (this *Module) Init() (err error) {
	return utils.Assert(this.Reload, this.Verify)
}

func (this *Module) Start() (err error) {
	return xserver.Start()
}

func (this *Module) Reload() (err error) {
	return options.Initialize()
}

func (this *Module) Verify() (err error) {
	if options.Options.Appid == "" {
		return errors.New("appid empty")
	}
	if options.Options.Secret == "" {
		return errors.New("secret empty")
	}
	var t *times.Times
	t, err = times.Parse(options.Options.Game.Time)
	if err != nil {
		return err
	}
	options.Game.ServerTime = t.Unix()
	return
}
