package config

import "github.com/hwcer/logger"

// Process 各 Handle 在 Reload 期产出的预处理表(概率表、索引等)，随快照整体发布。
type Process map[string]any

// Get 读取预处理产物。任意 goroutine 可安全调用(快照发布后只读)。
func (p Process) Get(name string) any {
	return p[name]
}

// Set 写入预处理产物。
//
// 🔴 只允许在 Reload 构建期(快照发布之前)由 Handle 调用：对已发布快照调用 Set
// 等于对在线 map 的并发写 —— runtime fatal 且不可 recover。要更新数据就构建
// 新快照整体发布。
func (p Process) Set(name string, value any) {
	if _, ok := p[name]; ok {
		logger.Error("SetProcess name exist:%s", name)
		return
	}
	p[name] = value
}
