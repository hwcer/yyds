package player

import (
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/emitter"
	"github.com/hwcer/updater/verify"
	"github.com/hwcer/uuid"
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
	uid    uint64
	uuid   *uuid.UUID
	Conn   net.Conn
	Role   *Role
	Task   *Task
	Items  *Items
	Active *Active
	Status int32 //在线状态
	//Notify  Notify  //通知
	//Battle  *Battle //当前战斗
	Expire  *Expire
	Verify  *verify.Verify
	Emitter *emitter.Emitter
	Message *Message       //最后一次发包的 MESSAGE
	mutex   sync.Mutex     //底层自动使用锁，不要手动调用
	workers map[string]any //自定义处理程序,在player加载完成时统一初始化
	//readOnly   bool
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
	p.Active = NewActive(p)
	p.Expire = &Expire{p: p}
	p.Verify = verify.New(p.Updater)
	p.Emitter = emitter.New(p.Updater)
}
