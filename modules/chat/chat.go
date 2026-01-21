package chat

import (
	"sync/atomic"
	"time"

	"github.com/hwcer/yyds/players/player"
)

// 短连接聊天模块
// 实现了一个无锁环形缓冲区，用于高效存储和读取聊天消息
// 特点：
// 1. 无锁设计，使用原子操作保证并发安全
// 2. 环形缓冲区，自动管理内存
// 3. 支持消息过滤
// 4. 高效的新消息检查机制

func New(cap int) *Chat {
	// 确保容量大于0
	if cap <= 0 {
		cap = 1024 // 默认容量
	}
	i := &Chat{cap: cap}
	i.rows = make([]*Message, cap)
	return i
}

// Chat 聊天管理器
// 使用无锁环形缓冲区存储消息
// 注意：所有字段都应该通过原子操作访问，避免竞态条件
type Chat struct {
	cap  int        // 环形缓冲区大小
	rows []*Message // 环形缓冲区，存储消息的数组
	head uint64     // 头指针，指向最早的消息位置
	tail uint64     // 尾指针，指向下一个要存储的位置
}

// Add 添加消息
// 参数：
//
//	m: 要添加的消息
//
// 注意：
//  1. 如果消息为 nil，会直接返回
//  2. 如果消息的 Time 字段为 0，会自动设置为当前时间戳
//  3. 消息的 Id 字段会被自动设置为递增的值
//  4. 当缓冲区已满时，会自动覆盖最早的消息
func (this *Chat) Add(m *Message) {
	// 处理消息为 nil 的情况
	if m == nil {
		return
	}

	// 检查消息是否过期（可选，根据业务需求）
	if m.Time == 0 {
		m.Time = time.Now().Unix()
	}

	// 原子更新尾指针并获取新值
	newTail := atomic.AddUint64(&this.tail, 1)

	// 使用 newTail 作为消息ID
	m.Id = newTail
	tail := newTail - 1
	// 存储消息到新位置（使用 newTail-1 作为索引）
	this.rows[tail%uint64(this.cap)] = m

	// 检查是否需要移动头指针
	head := atomic.LoadUint64(&this.head)
	if newTail-head > uint64(this.cap) {
		// 缓冲区已满，移动头指针
		atomic.AddUint64(&this.head, 1)
	}
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
// 注意：
//  1. 如果 t >= 当前最大ID，会返回空列表
//  2. 如果 size <= 0，会使用默认值50
//  3. 如果 size > 100，会被限制为100
//  4. 消息按时间倒序排列（最新的在前）
func (this *Chat) Read(t uint64, size int, filter Filter) (n uint64, r []*Message) {
	// 获取当前最大索引
	n = atomic.LoadUint64(&this.tail)
	if t >= n {
		return
	}

	// 限制返回消息数量
	if size <= 0 {
		size = 50
	} else if size > 100 {
		size = 100
	}

	// 预分配返回切片，减少内存分配
	r = make([]*Message, 0, size)

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
		// 移动到前一个位置
		current = (current - 1 + uint64(this.cap)) % uint64(this.cap)

		m := rows[current]
		if m == nil {
			continue
		}

		if m.Id <= t {
			break
		}

		if filter != nil && !filter(m) {
			continue
		}

		r = append(r, m)
	}
	return
}

// Index 获取当前最大消息ID
// 返回值：
//
//	当前最大消息ID，可用于下次拉取消息时的过滤
func (this *Chat) Index() uint64 {
	return atomic.LoadUint64(&this.tail)
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
// 注意：
//  1. 如果设置了频道，此时只能做模糊检查，用于红点提示
//  2. 内部通过比较玩家存储的最后消息ID和当前最大ID来计算
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
// 注意：
//  1. 如果 size < 10，会使用默认值10
//  2. 如果 size > 100，会被限制为100
//  3. 会自动更新玩家存储的最后消息ID
func (this *Chat) Getter(p *player.Player, size int, filter Filter) []*Message {
	if size < 10 {
		size = 10
	} else if size > 100 {
		size = 100
	}
	n := p.Values.GetInt64(NotifyName)
	nw, rows := this.Read(uint64(n), size, filter)
	if len(rows) > 0 {
		p.Values.Set(NotifyName, nw)
	}
	return rows
}
