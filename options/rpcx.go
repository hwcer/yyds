package options

var Service = map[string]string{}

var Rpcx = &rpcx{
	Timeout:             2,
	Network:             "tcp",
	Address:             ":8100",
	BasePath:            "rpcx",
	ClientMessageChan:   300,
	ClientMessageWorker: 1,
}

type rpcx = struct {
	Redis               string //服务发现
	Timeout             int32
	Network             string
	Address             string //仅仅启动服务器时需要
	BasePath            string
	ClientMessageChan   int //双向通信客户端接受消息通道大小
	ClientMessageWorker int //双向通信客户端处理消息协程数量
}