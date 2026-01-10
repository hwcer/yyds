package channel

import (
	"sync"

	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/logger"
)

var manage = sync.Map{}

func Get(name, value string) (r *Channel) {
	rk := Name(name, value)
	if i, ok := manage.Load(rk); ok {
		r = i.(*Channel)
	}
	return
}

func loadOrCreate(name, value string, fixed bool) (r *Channel) {
	rk := Name(name, value)
	newChannel := New(rk, fixed)
	if i, loaded := manage.LoadOrStore(rk, newChannel); loaded {
		r = i.(*Channel)
	} else {
		r = newChannel
	}
	return
}

// Join 将玩家加入指定名称和参数的频道
// 参数:
//
//	p - 玩家会话数据
//	name - 频道名称
//	value - 频道参数
//
// 注意: 同一名称的频道，一个玩家只能加入一个
func Join(p *session.Data, name string, value string) {
	logger.Debug("channel Join name:%s value:%s", name, value)
	setter := NewSetter(p)
	if old, ok := setter.Join(name, value); ok && old != value {
		leave(p, name, old)
	}
	room := loadOrCreate(name, value, false)
	if room == nil {
		logger.Error("channel Join failed: room creation error name:%s value:%s", name, value)
		return
	}
	room.Join(p)
}

// Leave 将玩家从指定频道移除
// 参数:
//
//	p - 玩家会话数据
//	name - 频道名称
//	value - 频道参数
func Leave(p *session.Data, name string, value string) {
	setter := NewSetter(p)
	setter.Leave(name)
	leave(p, name, value)
}

func leave(p *session.Data, name, value string) {
	logger.Debug("channel Leave name:%s value:%s", name, value)
	if room := Get(name, value); room != nil {
		room.Leave(p)
	}
}
func Range(name, value string, f func(*session.Data) bool) {
	room := Get(name, value)
	if room == nil {
		return
	}
	room.Range(f)
}

// Release 用户掉线,销毁时 清理所在房间信息
func Release(p *session.Data) {
	setter := NewSetter(p)
	rs := setter.Release()
	for _, r := range rs {
		leave(p, r.k, r.v)
	}
}

// Delete 销毁房间
func Delete(name, value string) {
	rk := Name(name, value)
	i, loaded := manage.LoadAndDelete(rk)
	if !loaded {
		return
	}
	room := i.(*Channel)
	room.Release()
}
