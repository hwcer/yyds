package kernel

import (
	"errors"
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosrpc/xshare"
	_ "github.com/hwcer/yyds/kernel/config"
	_ "github.com/hwcer/yyds/kernel/context"
	_ "github.com/hwcer/yyds/kernel/itypes"
	"github.com/hwcer/yyds/kernel/model"
	"github.com/hwcer/yyds/kernel/players"
	"github.com/hwcer/yyds/kernel/share"
	"strconv"
	"strings"
)

var mod *Module

func init() {
	mod = &Module{}
}

func New() *Module {
	return mod
}

type Module struct {
	cosgo.Module
}

func (this *Module) Id() string {
	return "kernel"
}
func (this *Module) Init() (err error) {
	if err = options.Initialize(); err != nil {
		return err
	}
	var ip string
	if ip, err = xshare.LocalIpv4(); err != nil {
		return
	}
	if options.Options.Debug {
		if options.Game.Sid == 0 {
			options.Game.Sid = autoServerId(ip)
		}
		if options.Game.Address == "" {
			addr := xshare.Address()
			if addr.Host == "0.0.0.0" {
				options.Game.Address = fmt.Sprintf("%v:%v", ip, addr.Port)
			} else {
				options.Game.Address = addr.String()
			}
		}
	}

	if options.Game.Sid == 0 {
		return errors.New("share.Options.Game.Sid empty")
	}
	if options.Game.Address == "" {
		return errors.New(" share.Options.Game.Address empty")
	} else {
		uri := utils.NewAddress(options.Game.Address)
		if uri.Scheme == "" {
			uri.Scheme = "http"
		}
		if uri.Empty() {
			uri.Host = ip
		}
		options.Game.Address = uri.String(true)
	}

	args := map[string]any{
		"sid":     options.Game.Sid,
		"name":    options.Game.Name,
		"address": options.Game.Address,
	}

	if options.Game.Notify != "" {
		uri := utils.NewAddress(options.Game.Notify)
		if uri.Scheme == "" {
			uri.Scheme = "http"
		}
		if uri.Empty() {
			uri.Host = ip
		}
		args["notify"] = uri.String(true)
	}

	if err = share.Master.Post(share.MasterApiTypeGameServerUpdate, args, nil); err != nil {
		if errors.Is(err, share.ErrMasterEmpty) {
			logger.Alert("配置项[master]为空,部分功能无法使用")
		} else {
			return fmt.Errorf(err.Error()+"，当前回调地址:%v", options.Game.Notify)
		}
	}
	return utils.Assert(model.Start, players.Start)
}

func (this *Module) Start() error {
	return nil
}

func (this *Module) Close() (err error) {
	if err = model.Close(); err != nil {
		return
	}
	return nil
}

func autoServerId(ip string) (sid int32) {
	ips := strings.Split(ip, ".")
	var pos uint = 8
	for i := 2; i <= 3; i++ {
		tempInt, _ := strconv.Atoi(ips[i])
		sid += int32(tempInt << pos)
		pos -= 8
	}
	return
}
