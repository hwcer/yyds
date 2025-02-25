package options

var Game = &game{}

type game = struct {
	Sid       int32  `json:"sid"`
	Time      string `json:"time"`    //开服时间
	Name      string `json:"name"`    //服务器名称
	Local     string `json:"local"`   //内网IP
	Redis     string `json:"redis"`   //排行榜
	Mongodb   string `json:"mongodb"` //数据库
	Address   string `json:"address"` //网关地址
	Developer bool   `json:"developer"`
	timeUnix  int64  //开服时间 int64
}
