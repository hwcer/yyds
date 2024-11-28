package player

import (
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/kernel/players/emitter"
	"github.com/hwcer/yyds/kernel/players/verify"
	"net"
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
	uid        uint64
	uuid       *uuid.UUID
	Conn       net.Conn
	Role       *Role
	Task       *Task
	Items      *Items
	Status     int32 //在线状态
	Expire     *Expire
	Verify     *verify.Verify
	Message    *Message   //最后一次发包的 MESSAGE
	mutex      sync.Mutex //底层自动使用锁，不要手动调用
	emitter    *emitter.Emitter
	heartbeat  int64 //最后心跳时间
	lastUpdate int64 //强制更新时间节点
	mustUpdate bool  //是否需要强制更新
}

func (p *Player) initialize() {
	if p.Role != nil {
		return
	}
	p.Role = NewRole(p)
	p.Task = NewTask(p)
	p.Items = NewItems(p)
	p.Expire = &Expire{p: p}
	p.Verify = verify.New(p.Updater)
	p.emitter = emitter.New(p.Updater)
}
