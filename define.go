package yyds

import (
	"strings"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/options"
)

const (
	FlagNameAppid  = "appid"
	FlagNameSecret = "secret"
	FlagNameMaster = "master"
)

func init() {
	cosgo.Config.Flags(FlagNameAppid, "", "", "游戏ID")
	cosgo.Config.Flags(FlagNameSecret, "", "", "游戏秘钥")
	cosgo.Config.Flags(FlagNameMaster, "", "", "Master服务器地址")
	cosgo.Config.SetDefault("pprof", "") //开启性能分析工具
	cosgo.On(cosgo.EventTypStarted, func() error {
		var s []string
		cosgo.Range(func(m cosgo.Module) bool {
			s = append(s, m.Id())
			return true
		})
		logger.Trace("启动模块:%v", strings.Join(s, ","))
		if options.Options.Master != "" {
			logger.Trace("中控地址:%v", options.Options.Master)
		}
		logger.Trace("服务器启动成功,请再次确认参数,Debug:%v, Appid:%v", cosgo.Debug(), options.Options.Appid)
		return nil
	})
}
