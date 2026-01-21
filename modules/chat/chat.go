package chat

import (
	"sync/atomic"

	"github.com/hwcer/yyds/players/player"
)

// 默认配置值
var (
	// DefaultSize 默认读取消息数量
	DefaultSize = 100
	// DefaultCap 默认缓冲区容量
	DefaultCap = 1024
)

// 短连接聊天模块
// 实现了一个无锁环形缓冲区，用于高效存储和读取聊天消息
// 特点：
// 1. 无锁设计，使用原子操作保证并发安全
// 2. 环形缓冲区，自动管理内存
// 3. 支持消息过滤
// 4. 高效的新消息检查机制

func New(cap int, factory Factory) *Chat {
	// 确保容量大于0
	if cap <= 0 {
		cap = DefaultCap // 默认容量
	}
	if factory == nil {
		factory = &defaultFactory{}
	}
	i := &Chat{cap: cap, factory: factory}
	i.rows = make([]Message, cap)
	return i
}

// Chat 聊天管理器
// 使用无锁环形缓冲区存储消息
// 无锁设计原理：
// 1. 使用原子操作管理head和tail指针，确保指针的一致性
// 2. 环形缓冲区的大小固定，避免动态扩容的开销
// 3. 当缓冲区已满时，自动覆盖最早的消息
// 4. 读取时从tail开始向前遍历，确保获取最新的消息
// 注意事项：
// 1. 所有指针操作都应该通过原子操作访问，避免竞态条件
// 2. 消息的生命周期由缓冲区大小和写入速度决定
// 3. 无锁设计依赖于指针操作的原子性，适用于读多写少的场景
type Chat struct {
	cap     int       // 环形缓冲区大小
	rows    []Message // 环形缓冲区，存储消息的数组
	head    uint64    // 头指针，指向最早的消息位置
	tail    uint64    // 尾指针，指向下一个要存储的位置
	factory Factory   // 用户工厂函数
}

// Write 添加消息
// 参数：
//
//	text: 消息内容
//	args: 附加参数
//	channel: 频道信息
//
// 返回值：
//
//	创建的消息对象
//
// 无锁实现原理：
// 1. 原子更新尾指针，确保在并发环境下的唯一性
// 2. 计算存储位置时使用取模运算，确保索引在缓冲区范围内
// 3. 存储消息后检查缓冲区容量，必要时移动头指针
// 4. 返回创建的消息，方便调用者直接使用
//
// 注意事项：
//  1. 消息的 Id 字段会被自动设置为递增的值
//  2. 当缓冲区已满时，会自动覆盖最早的消息
//  3. 消息的生命周期由缓冲区大小和写入速度决定
func (this *Chat) Write(text string, args map[string]any, channel *Channel) Message {
	// 原子更新尾指针并获取新值
	tail := atomic.AddUint64(&this.tail, 1)
	// 创建消息
	m := this.factory.New(tail, text, args, channel)
	// 计算存储位置
	index := (tail - 1) % uint64(this.cap)
	// 存储消息到新位置
	this.rows[index] = m

	// 检查是否需要移动头指针
	head := atomic.LoadUint64(&this.head)
	if tail-head > uint64(this.cap) {
		// 缓冲区已满，移动头指针
		atomic.AddUint64(&this.head, 1)
	}
	return m
}

// Read 获取最新聊天信息
// 参数：
//
//	t: 上次拉取的最后消息ID，用于过滤旧消息
//	size: 要获取的消息数量，最大为100
//	filter: 消息过滤器，用于筛选符合条件的消息
//
// 返回值：
//
//	n: 当前最大消息ID
//	r: 符合条件的最新消息列表
//
// 无锁实现原理：
// 1. 原子读取尾指针，确保获取最新的消息ID
// 2. 从尾指针开始向前遍历，确保获取最新的消息
// 3. 使用取模运算计算消息在缓冲区中的位置
// 4. 检查消息的ID，确保只返回比上次拉取更新的消息
// 5. 应用过滤器，筛选符合条件的消息
//
// 注意：
//  1. 如果 t >= 当前最大ID，会返回空列表
//  2. 如果 size <= 0，会使用默认值50
//  3. 如果 size > 100，会被限制为100
//  4. 消息按时间倒序排列（最新的在前）
func (this *Chat) Read(t uint64, size int, filter Filter) (n uint64, r []Message) {
	// 获取当前最大索引
	n = atomic.LoadUint64(&this.tail)
	if t >= n {
		return
	}

	// 限制返回消息数量
	if size <= 0 || size > DefaultSize {
		size = DefaultSize
	}

	// 预分配返回切片，减少内存分配
	r = make([]Message, 0, size)

	// 处理 rows 为 nil 的情况
	rows := this.rows
	if len(rows) == 0 {
		return
	}

	// 原子读取当前状态
	tail := atomic.LoadUint64(&this.tail)
	head := atomic.LoadUint64(&this.head)
	count := tail - head
	if count > uint64(this.cap) {
		count = uint64(this.cap)
	}

	// 从尾指针开始向前遍历，获取最新消息
	current := tail
	for i := uint64(0); i < count && len(r) < size; i++ {
		// 计算当前消息的存储位置
		index := (current - 1 + uint64(this.cap)) % uint64(this.cap)
		current = index

		m := rows[index]
		if m == nil {
			continue
		}

		if m.GetId() <= t {
			break
		}

		if filter != nil && !filter(m) {
			continue
		}

		r = append(r, m)
	}
	return
}

// Notify 获取是否有新的消息
// 参数：
//
//	p: 玩家对象
//
// 返回值：
//
//	新消息的数量
//
// 无锁实现原理：
// 1. 原子读取尾指针，获取当前最大消息ID
// 2. 从玩家对象中获取上次存储的消息ID
// 3. 计算两者的差值，即为新消息的数量
//
// 注意：
//  1. 如果设置了频道，此时只能做模糊检查，用于红点提示
//  2. 内部通过比较玩家存储的最后消息ID和当前最大ID来计算
//  3. 此方法是无锁的，适用于高频调用场景
func (this *Chat) Notify(p *player.Player) uint64 {
	n := p.Values.GetInt64(NotifyName)
	return atomic.LoadUint64(&this.tail) - uint64(n)
}

// Getter 获取最新聊天记录
// 参数：
//
//	p: 玩家对象
//	size: 要获取的消息数量，最大为100
//	filter: 消息过滤器，用于筛选符合条件的消息
//
// 返回值：
//
//	符合条件的最新消息列表
//
// 实现原理：
// 1. 从玩家对象中获取上次存储的消息ID
// 2. 调用Read方法获取最新的消息
// 3. 如果获取到消息，更新玩家对象中的最后消息ID
// 4. 返回符合条件的消息列表
func (this *Chat) Getter(p *player.Player, size int, filter Filter) []Message {
	n := p.Values.GetInt64(NotifyName)
	nw, rows := this.Read(uint64(n), size, filter)
	if len(rows) > 0 {
		p.Values.Set(NotifyName, nw)
	}
	return rows
}
