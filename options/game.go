package options

import "github.com/hwcer/cosgo/values"

var Game = &game{Values: values.Values{}}

type game = struct {
	Sid     int32  `json:"sid"`
	Unix    int64  `json:"-"`       //开服时间 int64
	Time    string `json:"time"`    //开服时间
	Name    string `json:"name"`    //服务器名称
	Local   string `json:"local"`   //内网IP
	Redis   string `json:"redis"`   //排行榜
	Mongodb string `json:"mongodb"` //数据库
	Address string `json:"address"` //网关地址
	//扩展参数,业务层自定义键名。本地可在 config.toml 的 [game.values] 中配置,
	//向 master 上报启动(/server/start)后,master 下发的同名键会覆盖本地值。
	Values values.Values `json:"values"`
	//Developer bool   `json:"developer"`
}
