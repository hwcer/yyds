package yyds

import (
	"fmt"
	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players"
	"strconv"
	"strings"
	"time"
)

var mod *Module

func init() {
	mod = &Module{}
	cosgo.On(cosgo.EventTypStarted, func() error {
		logger.Trace("当前服务器编号：%v", options.Game.Sid)
		logger.Trace("当前服务器地址：%v", options.Game.Local)
		logger.Trace("当前服务器时间：%v", times.Format())
		return nil
	})
}

func New() *Module {
	return mod
}

type Module struct {
}

func (this *Module) Id() string {
	return "yyds"
}
func (this *Module) Init() (err error) {
	if t := time.Now(); t.IsZero() {
		return errors.New("启动失败,无法获取系统时间")
	}

	if err = options.Initialize(); err != nil {
		return err
	}
	if options.Options.Appid == "" {
		return errors.New("appid empty")
	}
	
	addr := xshare.Address()
	if options.Game.Local == "" {
		options.Game.Local = addr.Local()
	}
	if utils.LocalValid(options.Game.Local) {
		return errors.New("无法自动获取内网ip或者内网ip配置错误")
	}
	if options.Game.Time != "" {
		var t *times.Times
		if t, err = times.Parse(options.Game.Time); err != nil {
			return err
		} else if t != nil {
			options.Game.Unix = t.Now().Unix()
		}
	}
	if options.Options.Debug {
		if options.Game.Sid == 0 {
			options.Game.Sid = autoServerId(options.Game.Local)
		}
		if options.Game.Address == "" {
			gate := utils.NewAddress(options.Gate.Address)
			if !gate.Valid() {
				gate.Host = options.Game.Local
			}
			options.Game.Address = gate.String()
		}
	}

	if options.Game.Sid == 0 {
		return errors.New("share.Options.Game.Sid empty")
	}
	if options.Game.Address == "" {
		return errors.New(" share.Options.Game.Address empty")
	}

	args := map[string]any{
		"sid":     options.Game.Sid,
		"name":    options.Game.Name,
		"local":   fmt.Sprintf("%s:%d", options.Game.Local, addr.Port),
		"address": options.Game.Address,
	}

	if err = options.Master.Post(options.MasterApiTypeGameServerStart, args, nil); err != nil {
		if errors.Is(err, errors.ErrMasterEmpty) {
			logger.Alert("配置项[master]为空,部分功能无法使用")
		} else {
			return fmt.Errorf(err.Error()+"，当前回调地址:%v", args["local"])
		}
	}
	//设置游戏Metadata
	xshare.Metadata.Set(options.ServiceTypeGame, fmt.Sprintf("%v=%v", options.SelectorServerId, options.Game.Sid))
	cosgo.On(cosgo.EventTypLoaded, players.Start)
	return nil
}

func (this *Module) Start() error {
	return nil
}

func (this *Module) Close() (err error) {
	args := map[string]any{
		"sid": options.Game.Sid,
	}
	if err = options.Master.Post(options.MasterApiTypeGameServerClose, args, nil); err != nil && !errors.Is(err, errors.ErrMasterEmpty) {
		logger.Alert("配置项[master]为空,部分功能无法使用:%v", err)
	}
	return nil
}

func autoServerId(ip string) (sid int32) {
	if i := strings.Index(ip, ":"); i >= 0 {
		ip = ip[:i-1]
	}
	ips := strings.Split(ip, ".")
	var pos uint = 8
	for i := 2; i <= 3; i++ {
		tempInt, _ := strconv.Atoi(ips[i])
		sid += int32(tempInt << pos)
		pos -= 8
	}
	return
}
