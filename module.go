package yyds

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hwcer/cosgo"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc"
	"github.com/hwcer/cosrpc/selector"
	"github.com/hwcer/cosrpc/server"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players"
)

var mod *Module

func init() {
	mod = &Module{}
	cosgo.On(cosgo.EventTypStarted, func() error {
		logger.Trace("当前服务器编号：%d", options.Game.Sid)
		logger.Trace("当前服务器地址：%s", options.Game.Local)
		logger.Trace("当前服务器时间：%s", times.Format())
		return nil
	})
}

func New() *Module {
	return mod
}

// ServerStartHandle master /server/start 回包处理器,业务层赋值给 Module.ServerStartHandle
//
// master 在启动上报的回包里下发本服的权威运营状态(可见性、维护、是否开放注册等),
// 业务层用它覆盖 [game] 里配的开机默认值 —— 启动路径上只此一次往返,
// 不必再单独拉一次 /server/info。
//
// 回包不在框架层解析,原样递过去,业务层 reply.Unmarshal(&v) 解到自己的结构即可:
// 字段是业务概念,yyds 不该认识它们。两件事业务层必须自己扛:
//   - 回包格式历来不统一(老版本 master 回的是 true),Unmarshal 失败属正常情况,
//     应当降级为沿用配置默认值而不是让启动失败;
//   - 回包可能没有 data(msg 只有 code),此时 Unmarshal 不报错、目标结构保持零值,
//     直接拿去覆盖开关就会把默认值抹成零 —— 业务层要按自己的字段(如 sid)校验有效性。
//
// 时机:Module.Init 内、开机默认值已灌进 players 之后,早于 players.Start,
// 因此回调里改运营开关不会有"按默认值裸奔"的窗口。回调返回 error 会中断启动。
// master 未配置或上报失败时不触发,业务层保持 [game] 的默认值。

type ServerStartHandle func(*values.Message) error

type Module struct {
	ServerStartHandle ServerStartHandle
}

func (this *Module) Id() string {
	return "yyds"
}
func (this *Module) Init() (err error) {
	if err = options.Initialize(); err != nil {
		return err
	}
	if options.Options.Appid == "" {
		return errors.New("appid empty")
	}

	addr := cosrpc.Address()
	if options.Game.Local == "" {
		options.Game.Local = addr.Local()
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
			options.Game.Sid, err = autoServerId()
		}
		if err != nil {
			return err
		}
		//if options.Game.Address == "" {
		//	gate := utils.NewAddress(options.Gate.Address)
		//	if !gate.Valid() {
		//		gate.Host = options.Game.Local
		//	}
		//	options.Game.Address = gate.String()
		//}
	}

	if options.Game.Sid == 0 {
		return errors.New("share.Options.Game.Sid empty")
	}
	if options.Game.Address == "" {
		logger.Alert("游戏服务器网关地址为空,客户端无法通过游戏平台获取游戏服务器地址")
	}

	//运营开关先按本地配置置位,再向 master 上报 —— 上报可能失败、也可能耗时,
	//这期间开关必须已经是确定值,否则会有一段"按零值裸奔"的窗口。
	//master 的权威值随上报回包一起下来,由业务层在 ServerStartHandle 里覆盖
	players.Maintain(options.Game.Maintain)
	players.Creatable(options.Game.Creatable)

	args := map[string]any{
		"sid":     options.Game.Sid,
		"name":    options.Game.Name,
		"local":   fmt.Sprintf("%s:%d", options.Game.Local, addr.Port),
		"address": options.Game.Address,
	}

	//回包在这一层不解析:老版本 master 回的是 true,新版本回的是服务器状态对象,
	//格式不统一,用 *values.Message 原样兜住再交给业务层(见 OnServerStart)。
	//原先把回包 merge 进 options.Game.Values 那套已移除:那是个普通 map,
	//master 推送写与玩家请求读并发访问会直接触发 Go 的并发读写 fatal,
	//而它承载的两个开关(维护/是否开放注册)现在都由 players 提供并发安全的接口
	reply := &values.Message{}
	if err = options.Master.Post(options.MasterApiTypeGameServerStart, args, reply); err != nil {
		if errors.Is(err, errors.ErrMasterEmpty) {
			logger.Alert("配置项[master]为空,部分功能无法使用")
		} else {
			return fmt.Errorf("%s，当前回调地址:%v", err.Error(), args["local"])
		}
	} else if this.ServerStartHandle != nil {
		if err = this.ServerStartHandle(reply); err != nil {
			return err
		}
	}
	//设置游戏Metadata
	server.Metadata.Set(options.ServiceTypeGame, fmt.Sprintf("%v=%v", selector.MetaDataServerId, options.Game.Sid))
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

func autoServerId() (sid int32, err error) {
	ip, err := utils.LocalIPv4()
	if err != nil {
		return 0, err
	}
	if i := strings.Index(ip, ":"); i >= 0 {
		ip = ip[:i]
	}
	ips := strings.Split(ip, ".")
	if len(ips) < 4 {
		return 0, fmt.Errorf("invalid IPv4 address: %s", ip)
	}
	var pos uint = 8
	for i := 2; i <= 3; i++ {
		tempInt, err := strconv.Atoi(ips[i])
		if err != nil {
			return 0, fmt.Errorf("invalid IPv4 segment: %s", ips[i])
		}
		sid += int32(tempInt << pos)
		pos -= 8
	}
	return
}
