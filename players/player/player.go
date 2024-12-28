package player

import (
	"github.com/hwcer/cosgo/uuid"
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

func New(uid uint64) *Player {
	return &Player{uid: uid}
}

type Player struct {
	*updater.Updater
	uid       uint64
	uuid      *uuid.UUID
	Times     *Times
	Lively    int64 //登录时间
	Status    int32 //在线状态
	Verify    *verify.Verify
	Emitter   *emitter.Emitter
	Message   *Message   //最后一次发包的 MESSAGE
	Gateway   string     //网关地址
	Session   string     //当前登录session id
	mutex     sync.Mutex //底层自动使用锁，不要手动调用
	heartbeat int64      //最后心跳时间
}

func (p *Player) initialize() {
	if p.Times != nil {
		return
	}
	p.Times = &Times{p: p}
	p.Verify = verify.New(p.Updater)
	p.Emitter = emitter.New(p.Updater)
}
