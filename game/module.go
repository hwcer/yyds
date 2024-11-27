package game

import (
	"errors"
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/game/players"
	"github.com/hwcer/yyds/game/share"
	"strconv"
	"strings"
)

var mod *Module

func init() {
	mod = &Module{Module: cosgo.Module{Id: "game"}}
}

func New() *Module {
	return mod
}

type Module struct {
	cosgo.Module
}

func (this *Module) Init() (err error) {
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
		"sid":     service.ServerId(),
		"name":    share.Options.Game.Name,
		"address": share.Options.Game.Address,
	}

	if share.Options.Game.Notify != "" {
		uri := utils.NewAddress(share.Options.Game.Notify)
		if uri.Scheme == "" {
			uri.Scheme = "http"
		}
		if uri.Empty() {
			uri.Host = ip
		}
		args["notify"] = uri.String(true)
	}

	if err = share.Master.Post(share.MasterApiTypeGameServerUpdate, args, nil); err != nil {
		if errors.Is(err, share.ErrMasterUrlEmpty) {
			logger.Trace(err)
		} else {
			return fmt.Errorf(err.Error()+"，当前回调地址:%v", share.Options.Game.Notify)
		}
	}
	if err = model.Start(); err != nil {
		return
	}
	if err = players.Start(); err != nil {
		return
	}
	if err = service.Start(); err != nil {
		return
	}
	return
}

func (this *Module) Start() error {
	if model.Redis != nil {
		//if err := rank2.Start(model.Redis); err != nil {
		//	return err
		//} else {
		//	logger.Trace("Redis排行榜功能已经启用")
		//}
	}
	if err := local.Start(); err != nil {
		return err
	}
	if err := handle.Start(); err != nil {
		return err
	}
	if err := master.Start(); err != nil {
		return err
	}
	logger.Trace("游戏服务启动完毕:%v", service.ServerId())
	return nil
}

func (this *Module) Close() (err error) {
	//if err = rank2.Close(); err != nil {
	//	return err
	//}
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
