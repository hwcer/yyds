package channel

import (
	"sync"

	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/logger"
)

// New 创建一个新的频道实例
// 参数:
//
//	name - 频道ID
//	fixed - 是否为固定频道（固定频道不会自动删除）
//
// 返回值:
//
//	新创建的频道实例
func New(name string, fixed bool) *Channel {
	return &Channel{id: name, fixed: fixed, ps: map[string]*session.Data{}}
}

type Channel struct {
	id       string
	ps       map[string]*session.Data
	fixed    bool //固定频道不会自动删除
	locker   sync.RWMutex
	released bool //已经删除 无法进入
}

func (this *Channel) Id() string {
	return this.id
}

func (this *Channel) Join(d *session.Data) bool {
	// 快速路径检查：使用读锁检查玩家是否已经在频道中
	this.locker.RLock()
	exists := this.ps[d.UUID()] != nil
	released := this.released
	this.locker.RUnlock()

	if exists {
		return true
	}
	if released {
		return false
	}

	this.locker.Lock()
	defer this.locker.Unlock()
	// 双重检查，避免加锁期间其他协程已经添加了该玩家或频道被释放
	if this.released {
		return false
	}
	if _, exists := this.ps[d.UUID()]; exists {
		return true
	}
	this.ps[d.UUID()] = d
	return true
}

func (this *Channel) Leave(d *session.Data) bool {
	this.locker.Lock()
	defer this.locker.Unlock()

	// 检查玩家是否在频道中
	if _, exists := this.ps[d.UUID()]; !exists {
		return false
	}

	delete(this.ps, d.UUID())
	if !this.fixed && len(this.ps) == 0 {
		this.released = true
		manage.Delete(this.id)
		logger.Debug("人数为空，房间销毁:%s", this.id)
	}
	return true
}

func (this *Channel) Release() {
	this.locker.Lock()
	defer this.locker.Unlock()
	this.released = true
	this.removeAllPlayer()
	//manage.Delete(this.id)
}

// removeAllPlayer 房间销毁时，清理所有房间内的成员
// 注意：该方法只能在已获取写锁的情况下调用
func (this *Channel) removeAllPlayer() {
	k, _ := Split(this.id)
	for _, d := range this.ps {
		setter := NewSetter(d)
		setter.Leave(k)
	}
}

func (this *Channel) Range(f func(*session.Data) bool) {
	// 先获取所有玩家的副本，然后在锁外执行回调
	var players []*session.Data
	this.locker.RLock()
	players = make([]*session.Data, 0, len(this.ps)) // 预分配内存
	for _, p := range this.ps {
		players = append(players, p)
	}
	this.locker.RUnlock()

	// 在锁外遍历并调用回调
	for _, p := range players {
		if !f(p) {
			return
		}
	}
}

func (this *Channel) Broadcast(path string, data []byte) {
	this.Range(func(p *session.Data) bool {
		SendMessage(p, path, data)
		return true
	})
}
