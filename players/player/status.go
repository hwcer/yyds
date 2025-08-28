package player

const (
	StatusNone       int32 = iota //初始仅仅被初始化到内存,启动服务器或者异步操作读取到内存
	StatusLocked                  //临时锁定状态
	StatusConnected               //上线
	StatusDisconnect              //断开连接
	StatusOffline                 //掉线  进入回收队列,此时上线还能抢救一下
	StatusReleased                //正在释放资源,此时无法进行任何操作
)
