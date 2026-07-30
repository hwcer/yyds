package options

// Creatable 默认 true:新服开出来就该能创角;Maintain 零值 false 即正常提供服务。
// 配置里没写这两个键时 mapstructure 不会清零,预设值得以保留
var Game = &game{Creatable: true}

type game = struct {
	Sid     int32  `json:"sid"`
	Unix    int64  `json:"-"`       //开服时间 int64
	Time    string `json:"time"`    //开服时间
	Name    string `json:"name"`    //服务器名称
	Local   string `json:"local"`   //内网IP
	Redis   string `json:"redis"`   //排行榜
	Mongodb string `json:"mongodb"` //数据库
	Address string `json:"address"` //网关地址
	//运营开关的初值,启动时(向 master 上报之前)灌进 players。
	//运行期由 master 推送或 GM 指令改,改的是 players 里的原子量,不回写这里 ——
	//这里只是"开机默认值",不是运行期的权威值,别拿它当状态读
	Maintain  bool `json:"maintain"`  //是否以维护状态启动
	Creatable bool `json:"creatable"` //是否允许创建新角色
	//Developer bool   `json:"developer"`
}
