package graph

import (
	"fmt"
	"runtime"
	"unsafe"
)

type UserMemoryUsage interface {
	MemoryUsage() uintptr
}

// PrintMemoryUsage 结构体中添加内存统计方法
func (sg *Graph) PrintMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 计算结构体本身大小
	graphSize := unsafe.Sizeof(*sg)
	nodesSize := unsafe.Sizeof(sg.nodes)
	for id, u := range sg.nodes {
		nodesSize += unsafe.Sizeof(id)
		nodesSize += unsafe.Sizeof(u)
		if mu, ok := u.(UserMemoryUsage); ok {
			nodesSize += mu.MemoryUsage()
		}
	}

	friendsSize := unsafe.Sizeof(sg.friends)
	for id, friends := range sg.friends {
		friendsSize += unsafe.Sizeof(id)
		friendsSize += unsafe.Sizeof(friends)
		for k, u := range friends {
			friendsSize += unsafe.Sizeof(k)
			friendsSize += unsafe.Sizeof(u)
		}
	}

	totalEstimated := graphSize + nodesSize + friendsSize

	fmt.Println("\nGraph Memory Usage:")
	fmt.Printf("Graph structure: %d bytes\n", graphSize)
	fmt.Printf("Nodes map: %.2f MB (%d users)\n", float64(nodesSize)/1024/1024, len(sg.nodes))
	fmt.Printf("Friends relationships: %d bytes (%.2f MB)\n", friendsSize, float64(friendsSize)/1024/1024)
	fmt.Printf("Estimated total: %d bytes (%.2f MB)\n", totalEstimated, float64(totalEstimated)/1024/1024)

	// 实际内存统计
	fmt.Println("\nRuntime Memory Stats:")
	fmt.Printf("Allocated: %.2f MB\n", float64(m.Alloc)/1024/1024)
	fmt.Printf("Total allocated: %.2f MB\n", float64(m.TotalAlloc)/1024/1024)
	fmt.Printf("Heap objects: %d\n", m.HeapObjects)
}
