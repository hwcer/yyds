package player

import (
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/players/emitter"
	"github.com/hwcer/yyds/players/verify"
	"sync"
)

type Message struct {
	Id   int32  //req id
	Data []byte //*context.Message
}
type Handle func(*Player) error

func New(uid string) *Player {
	return &Player{uid: uid}
}

type Player struct {
	*updater.Updater
	uid       string
	mutex     sync.Mutex       //底层自动使用锁，不要手动调用
	heartbeat int64            //最后心跳时间
	Times     *Times           //时间控制器
	Dirty     Dirty            //短连接推送数据缓存
	Login     int64            //登录时间
	Binder    binder.Binder    //当前端使用的序列化方式
	Status    int32            //在线状态
	Verify    *verify.Verify   //全局条件验证
	Emitter   *emitter.Emitter //全局事件
	Message   *Message         //最后一次发包的 MESSAGE
	Gateway   uint64           //网关地址

}

func (p *Player) initialize() {
	if p.Times != nil {
		return
	}
	p.Times = &Times{p: p}
	p.Verify = verify.New(p.Updater)
	p.Emitter = emitter.New(p.Updater)
}
