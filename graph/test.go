package graph

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

type UserMemoryUsage interface {
	MemoryUsage() uintptr
}

// relation的内存统计方法(不递归计算Player)
func (r relation) memoryUsage() uintptr {
	if r == nil {
		return 0
	}

	var total uintptr

	// map结构本身的内存
	total += unsafe.Sizeof(r)

	// 每个键值对的内存
	// 注意: 这只是近似值，实际map实现可能有额外开销
	for key := range r {
		// 键的内存
		total += uintptr(len(key))
		// 值的指针内存(不递归)
		total += unsafe.Sizeof(&node{})
	}

	return total
}

// MemoryUsage 返回Graph实例及其所有内容占用的内存大小(字节)
func (g *Graph) MemoryUsage() uintptr {
	var total uintptr

	// 1. 计算Graph结构体本身的基础内存占用
	total += unsafe.Sizeof(*g)

	// 2. 计算互斥锁的内存占用
	total += unsafe.Sizeof(sync.RWMutex{})

	// 3. 计算nodes映射的结构内存
	g.mu.RLock()
	defer g.mu.RUnlock()

	total += unsafe.Sizeof(g.nodes) // 映射头部信息

	// 4. 计算每个节点及其内容的内存
	for key, player := range g.nodes {
		// 键的内存
		total += uintptr(len(key))

		// 玩家结构内存
		total += unsafe.Sizeof(*player)

		// 如果Player中的User实现了MemoryUsage接口，使用更精确的计算
		if umu, ok := player.Data.(UserMemoryUsage); ok {
			total += umu.MemoryUsage()
		} else {
			// 否则只计算User接口变量的大小
			total += unsafe.Sizeof(player.Data)
		}

		// 计算fans和friends关系的内存(只计算map结构和指针大小)
		total += player.fans.memoryUsage()
		total += player.friends.memoryUsage()
	}

	// 5. 计算factory的内存占用(假设Factory是一个接口)
	total += unsafe.Sizeof(g.factory)

	return total
}

// PrintMemoryUsage 结构体中添加内存统计方法
func (sg *Graph) PrintMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	totalEstimated := sg.MemoryUsage()

	fmt.Println("\nGraph Memory Usage:")
	fmt.Printf("Nodes map: %.2f MB (%d users)\n", float64(totalEstimated)/1024/1024, len(sg.nodes))

	// 实际内存统计
	fmt.Println("\nRuntime Memory Stats:")
	fmt.Printf("Allocated: %.2f MB\n", float64(m.Alloc)/1024/1024)
	fmt.Printf("Total allocated: %.2f MB\n", float64(m.TotalAlloc)/1024/1024)
	fmt.Printf("Heap objects: %d\n", m.HeapObjects)
}
