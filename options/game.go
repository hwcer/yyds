package options

var Game = &game{}

type game = struct {
	Sid       int32  `json:"sid"`
	Time      string `json:"time"`    //开服时间
	Name      string `json:"name"`    //服务器名称
	Redis     string `json:"redis"`   //排行榜
	Plugin    string `json:"plugin"`  //热更文件  plugin.so 相对于 worker dir
	Mongodb   string `json:"mongodb"` //数据库
	Notify    string `json:"notify"`  //管理地址
	Address   string `json:"address"` //网关地址
	Developer bool   `json:"developer"`
	timeUnix  int64  //开服时间 int64
}
