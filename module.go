package yyds

import (
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players"
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
	return "yyds"
}
func (this *Module) Init() (err error) {

	if err = utils.Assert(options.Initialize, this.Verify); err != nil {
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

	if err = options.Master.Post(options.MasterApiTypeGameServerUpdate, args, nil); err != nil {
		if errors.Is(err, errors.ErrMasterEmpty) {
			logger.Alert("配置项[master]为空,部分功能无法使用")
		} else {
			return fmt.Errorf(err.Error()+"，当前回调地址:%v", options.Game.Notify)
		}
	}
	return utils.Assert(players.Start)
}

func (this *Module) Start() error {
	return nil
}

func (this *Module) Close() (err error) {
	return nil
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
