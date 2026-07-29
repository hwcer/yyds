package gomcp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	runtimepprof "runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/pprof/profile"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 采集时长上限:AI 可能反复触发,必须封顶,避免长时间 CPU 采集拖慢服务。
const captureSecondsMax = 30

// 同一时刻只允许一个采集在跑(CPU 采集本身也不允许并发)。
var captureMutex sync.Mutex

func registerPprof(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "runtime_stats",
		Description: "获取 Go 运行时统计:内存占用/GC 情况/goroutine 数量/线程数。零采集开销,性能问题的第一跳。",
	}, runtimeStats)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "goroutine_dump",
		Description: "导出 goroutine 栈。mode=summary 按栈顶函数聚合计数(排查泄漏/卡死首选),mode=full 输出完整栈文本。",
	}, goroutineDump)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "pprof_capture",
		Description: "采集 pprof 性能剖析并返回 top-N 热点。type: cpu/heap/allocs/goroutine/block/mutex/threadcreate。原始 .pb.gz 同时落盘,可用 go tool pprof 深挖。",
	}, pprofCapture)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "pprof_diff",
		Description: "对比两次 pprof_capture 的结果(传各自返回的 file),按函数聚合输出增长最多的位置。定位内存泄漏用。",
	}, pprofDiff)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "pprof_enable",
		Description: "开关 block/mutex 剖析。二者默认关闭且常驻开启会拖慢服务,采集完记得关掉(传 0)。",
	}, pprofEnable)
}

// ------------------------------------------------------------------ runtime_stats

type runtimeArgs struct{}

type runtimeReply struct {
	Goroutine    int     `json:"goroutine"`
	Thread       int     `json:"thread"`
	CPU          int     `json:"cpu"`
	HeapAllocMB  float64 `json:"heapAllocMB"`  //当前堆上存活对象占用
	HeapSysMB    float64 `json:"heapSysMB"`    //从系统申请的堆内存
	HeapObjects  uint64  `json:"heapObjects"`  //存活对象数
	StackSysMB   float64 `json:"stackSysMB"`   //栈占用
	SysMB        float64 `json:"sysMB"`        //进程从系统获取的总内存
	TotalAllocMB float64 `json:"totalAllocMB"` //累计分配(含已释放)
	NumGC        uint32  `json:"numGC"`
	LastGC       string  `json:"lastGC"`
	GCPauseMs    float64 `json:"gcPauseMs"`    //最近一次 GC 暂停
	GCPauseAvgMs float64 `json:"gcPauseAvgMs"` //平均暂停
	GCCPUPercent float64 `json:"gcCpuPercent"` //GC 占用 CPU 比例
	NextGCMB     float64 `json:"nextGCMB"`     //下次 GC 触发阈值
	BlockProfile int     `json:"blockProfile"` //block 采样率,0=关闭
	MutexProfile int     `json:"mutexProfile"` //mutex 采样比,0=关闭
}

func runtimeStats(ctx context.Context, req *mcp.CallToolRequest, args runtimeArgs) (*mcp.CallToolResult, any, error) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	var pauseAvg float64
	if ms.NumGC > 0 {
		n := min(uint32(len(ms.PauseNs)), ms.NumGC)
		var sum uint64
		for i := uint32(0); i < n; i++ {
			sum += ms.PauseNs[(ms.NumGC-i-1)%uint32(len(ms.PauseNs))]
		}
		pauseAvg = float64(sum) / float64(n) / 1e6
	}
	var lastGC string
	var lastPause float64
	if ms.LastGC > 0 {
		lastGC = time.Unix(0, int64(ms.LastGC)).Format(time.DateTime)
		lastPause = float64(ms.PauseNs[(ms.NumGC+255)%256]) / 1e6
	}
	r := &runtimeReply{
		Goroutine:    runtime.NumGoroutine(),
		Thread:       threadCount(),
		CPU:          runtime.NumCPU(),
		HeapAllocMB:  mb(ms.HeapAlloc),
		HeapSysMB:    mb(ms.HeapSys),
		HeapObjects:  ms.HeapObjects,
		StackSysMB:   mb(ms.StackSys),
		SysMB:        mb(ms.Sys),
		TotalAllocMB: mb(ms.TotalAlloc),
		NumGC:        ms.NumGC,
		LastGC:       lastGC,
		GCPauseMs:    lastPause,
		GCPauseAvgMs: pauseAvg,
		GCCPUPercent: ms.GCCPUFraction * 100,
		NextGCMB:     mb(ms.NextGC),
		BlockProfile: blockRate,
		MutexProfile: mutexFraction,
	}
	return jsonResult(r)
}

func mb(v uint64) float64 {
	return float64(v) / 1024 / 1024
}

func threadCount() int {
	if p := runtimepprof.Lookup("threadcreate"); p != nil {
		return p.Count()
	}
	return 0
}

// ------------------------------------------------------------------ goroutine_dump

type goroutineArgs struct {
	Mode  string `json:"mode,omitempty" jsonschema:"取值 summary(按栈顶函数聚合计数,默认)或 full(完整栈文本)"`
	Limit int    `json:"limit,omitempty" jsonschema:"summary 模式下返回前 N 项,默认 30"`
}

func goroutineDump(ctx context.Context, req *mcp.CallToolRequest, args goroutineArgs) (*mcp.CallToolResult, any, error) {
	p := runtimepprof.Lookup("goroutine")
	if p == nil {
		return nil, nil, fmt.Errorf("goroutine profile 不可用")
	}
	if strings.EqualFold(args.Mode, "full") {
		buf := &bytes.Buffer{}
		if err := p.WriteTo(buf, 1); err != nil {
			return nil, nil, err
		}
		return text(buf.String()), nil, nil
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 30
	}
	buf := &bytes.Buffer{}
	if err := p.WriteTo(buf, 0); err != nil {
		return nil, nil, err
	}
	prof, err := profile.Parse(buf)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(map[string]any{
		"total":  p.Count(),
		"unit":   "goroutine",
		"detail": "按最内层业务函数聚合(跳过 runtime.gopark 等调度帧),caller 是它的调用者",
		"top":    goroutineTop(prof, limit),
	})
}

type goroutineItem struct {
	Function string `json:"function"`
	File     string `json:"file,omitempty"`
	Caller   string `json:"caller,omitempty"`
	Count    int64  `json:"count"`
}

// goroutineTop 按「最内层的非 runtime 帧」聚合。
//
// goroutine profile 的栈顶几乎总是 runtime.gopark/notetsleepg 这类调度帧,
// 直接按栈顶聚合的结果对排查毫无信息量(全都堆在 gopark 上)。
// 真正有价值的是「阻塞在哪段业务代码」,所以跳过 runtime.* 取第一个业务帧。
func goroutineTop(p *profile.Profile, limit int) []*goroutineItem {
	type agg struct {
		file   string
		caller string
		count  int64
	}
	m := map[string]*agg{}
	for _, s := range p.Sample {
		var fn, file, caller string
		for i, loc := range s.Location {
			name := funcOf(loc)
			if name == "" || strings.HasPrefix(name, "runtime.") {
				continue
			}
			fn, file = name, lineOf(loc)
			//往外再找一层作为调用者,便于定位是谁起的这个 goroutine
			for _, up := range s.Location[i+1:] {
				if c := funcOf(up); c != "" && !strings.HasPrefix(c, "runtime.") {
					caller = c
					break
				}
			}
			break
		}
		if fn == "" {
			fn = "runtime(调度中,无业务帧)"
		}
		a := m[fn]
		if a == nil {
			a = &agg{file: file, caller: caller}
			m[fn] = a
		}
		a.count += s.Value[0]
	}
	r := make([]*goroutineItem, 0, len(m))
	for fn, a := range m {
		r = append(r, &goroutineItem{Function: fn, File: a.file, Caller: a.caller, Count: a.count})
	}
	sort.Slice(r, func(i, j int) bool { return r[i].Count > r[j].Count })
	if len(r) > limit {
		r = r[:limit]
	}
	return r
}

// ------------------------------------------------------------------ pprof_capture

type captureArgs struct {
	Type    string `json:"type" jsonschema:"cpu/heap/allocs/goroutine/block/mutex/threadcreate"`
	Seconds int    `json:"seconds,omitempty" jsonschema:"仅 cpu 有效,采集时长秒数,默认 10,上限 30"`
	Limit   int    `json:"limit,omitempty" jsonschema:"返回前 N 个热点,默认 20"`
}

type captureReply struct {
	Type       string     `json:"type"`
	SampleType string     `json:"sampleType"`
	Unit       string     `json:"unit"`
	Total      int64      `json:"total"`
	File       string     `json:"file"` //原始 .pb.gz 路径,可用 go tool pprof 打开,也可传给 pprof_diff
	Note       string     `json:"note,omitempty"`
	Top        []*topItem `json:"top"`
}

// captureNote 解释「采到 0 个样本」,避免把空 top 误读成工具坏了。
func captureNote(name string) string {
	switch name {
	case "cpu":
		return "采集期间进程几乎没有占用 CPU(空闲服务器的正常结果),不是工具异常;要看热点请在有负载时采集"
	case "block", "mutex":
		return "该 profile 默认关闭,先用 pprof_enable 打开、跑一段业务后再采集"
	default:
		return "本次没有采集到样本"
	}
}

func pprofCapture(ctx context.Context, req *mcp.CallToolRequest, args captureArgs) (*mcp.CallToolResult, any, error) {
	name := strings.ToLower(strings.TrimSpace(args.Type))
	if name == "" {
		return nil, nil, fmt.Errorf("type 不能为空")
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 20
	}
	captureMutex.Lock()
	defer captureMutex.Unlock()

	buf := &bytes.Buffer{}
	if name == "cpu" {
		seconds := args.Seconds
		if seconds <= 0 {
			seconds = 10
		}
		if seconds > captureSecondsMax {
			seconds = captureSecondsMax
		}
		if err := runtimepprof.StartCPUProfile(buf); err != nil {
			return nil, nil, err
		}
		select {
		case <-time.After(time.Duration(seconds) * time.Second):
		case <-ctx.Done():
		}
		runtimepprof.StopCPUProfile()
	} else {
		p := runtimepprof.Lookup(name)
		if p == nil {
			return nil, nil, fmt.Errorf("未知的 profile 类型:%s(可用:cpu/heap/allocs/goroutine/block/mutex/threadcreate)", name)
		}
		if name == "heap" {
			runtime.GC() //取存活对象前先跑一次 GC,否则 inuse 含未回收垃圾,读数偏大
		}
		if err := p.WriteTo(buf, 0); err != nil {
			return nil, nil, err
		}
	}
	raw := buf.Bytes()
	prof, err := profile.ParseData(raw)
	if err != nil {
		return nil, nil, err
	}
	idx := defaultSampleIndex(prof)
	file, err := dump(name, raw)
	if err != nil {
		return nil, nil, err
	}
	st := prof.SampleType[idx]
	r := &captureReply{
		Type:       name,
		SampleType: st.Type,
		Unit:       st.Unit,
		Total:      totalOf(prof, idx),
		File:       file,
		Top:        topOf(prof, idx, limit),
	}
	if r.Total == 0 {
		r.Note = captureNote(name)
	}
	return jsonResult(r)
}

// ------------------------------------------------------------------ pprof_diff

type diffArgs struct {
	Base    string `json:"base" jsonschema:"基准 profile 文件路径(pprof_capture 返回的 file)"`
	Current string `json:"current" jsonschema:"对比 profile 文件路径"`
	Limit   int    `json:"limit,omitempty" jsonschema:"返回前 N 项,默认 20"`
}

type diffItem struct {
	Function string `json:"function"`
	Base     int64  `json:"base"`
	Current  int64  `json:"current"`
	Delta    int64  `json:"delta"`
}

func pprofDiff(ctx context.Context, req *mcp.CallToolRequest, args diffArgs) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 20
	}
	base, bi, err := load(args.Base)
	if err != nil {
		return nil, nil, err
	}
	cur, ci, err := load(args.Current)
	if err != nil {
		return nil, nil, err
	}
	if base.SampleType[bi].Type != cur.SampleType[ci].Type {
		return nil, nil, fmt.Errorf("两个 profile 类型不一致:%s vs %s", base.SampleType[bi].Type, cur.SampleType[ci].Type)
	}
	bm := flatOf(base, bi)
	cm := flatOf(cur, ci)
	r := make([]*diffItem, 0, len(cm))
	for fn, cv := range cm {
		bv := bm[fn]
		if d := cv - bv; d != 0 {
			r = append(r, &diffItem{Function: fn, Base: bv, Current: cv, Delta: d})
		}
	}
	//只关心增长,降序取前 N
	sort.Slice(r, func(i, j int) bool { return r[i].Delta > r[j].Delta })
	if len(r) > limit {
		r = r[:limit]
	}
	return jsonResult(map[string]any{
		"sampleType": cur.SampleType[ci].Type,
		"unit":       cur.SampleType[ci].Unit,
		"baseTotal":  totalOf(base, bi),
		"currTotal":  totalOf(cur, ci),
		"growth":     r,
	})
}

// ------------------------------------------------------------------ pprof_enable

var (
	blockRate     int
	mutexFraction int
)

type enableArgs struct {
	BlockRate     *int `json:"blockRate,omitempty" jsonschema:"block 采样率(纳秒),0=关闭。典型值 1000000(1ms)"`
	MutexFraction *int `json:"mutexFraction,omitempty" jsonschema:"mutex 采样比,0=关闭。典型值 5(采样 1/5)"`
}

func pprofEnable(ctx context.Context, req *mcp.CallToolRequest, args enableArgs) (*mcp.CallToolResult, any, error) {
	if args.BlockRate != nil {
		blockRate = *args.BlockRate
		runtime.SetBlockProfileRate(blockRate)
	}
	if args.MutexFraction != nil {
		mutexFraction = *args.MutexFraction
		runtime.SetMutexProfileFraction(mutexFraction)
	}
	return jsonResult(map[string]any{
		"blockProfile": blockRate,
		"mutexProfile": mutexFraction,
		"remind":       "采集完请把对应值设回 0,长期开启会拖慢服务",
	})
}

// ------------------------------------------------------------------ profile 解析

type topItem struct {
	Function string `json:"function"`
	File     string `json:"file,omitempty"`
	Flat     int64  `json:"flat"`
	FlatPerc string `json:"flatPerc"`
	Cum      int64  `json:"cum"`
}

// defaultSampleIndex 选择有意义的取值列。
// heap 的 DefaultSampleType 是 inuse_space,cpu 没有默认值时取最后一列(cpu/nanoseconds)。
func defaultSampleIndex(p *profile.Profile) int {
	if p.DefaultSampleType != "" {
		for i, st := range p.SampleType {
			if st.Type == p.DefaultSampleType {
				return i
			}
		}
	}
	return len(p.SampleType) - 1
}

func totalOf(p *profile.Profile, idx int) (r int64) {
	for _, s := range p.Sample {
		r += s.Value[idx]
	}
	return
}

// flatOf 按「最内层的非 runtime 帧」聚合,口径与 topOf 保持一致。
func flatOf(p *profile.Profile, idx int) map[string]int64 {
	r := map[string]int64{}
	for _, s := range p.Sample {
		for _, loc := range s.Location {
			fn := funcOf(loc)
			if fn == "" || strings.HasPrefix(fn, "runtime.") {
				continue
			}
			r[fn] += s.Value[idx]
			break
		}
	}
	return r
}

// topOf 计算热点。
//
// flat 归属到「最内层的非 runtime 帧」而不是字面栈顶:heap profile 的栈顶总是
// runtime.mallocgc、goroutine profile 的栈顶总是 runtime.gopark,按字面栈顶聚合
// 出来的 top 全是这些调度/分配帧,对定位业务代码毫无帮助。
func topOf(p *profile.Profile, idx, limit int) []*topItem {
	flat := map[string]int64{}
	cum := map[string]int64{}
	file := map[string]string{}
	for _, s := range p.Sample {
		v := s.Value[idx]
		//cum 按函数去重,同一函数在一条栈上出现多次(递归)只计一次
		seen := map[string]bool{}
		var flated, top string
		for _, loc := range s.Location {
			fn := funcOf(loc)
			if fn == "" {
				continue
			}
			if top == "" {
				top = fn
			}
			if flated == "" && !strings.HasPrefix(fn, "runtime.") {
				flated = fn
				flat[fn] += v
			}
			if !seen[fn] {
				seen[fn] = true
				cum[fn] += v
			}
			if _, ok := file[fn]; !ok {
				file[fn] = lineOf(loc)
			}
		}
		//整条栈都在 runtime 内(空闲进程的 CPU 采样常见),退回按真实栈顶归属,
		//否则这些样本会被全部丢弃、top 变成空列表
		if flated == "" && top != "" {
			flat[top] += v
		}
	}
	total := totalOf(p, idx)
	r := make([]*topItem, 0, len(flat))
	for fn, v := range flat {
		var perc string
		if total > 0 {
			perc = fmt.Sprintf("%.2f%%", float64(v)*100/float64(total))
		}
		r = append(r, &topItem{Function: fn, File: file[fn], Flat: v, FlatPerc: perc, Cum: cum[fn]})
	}
	sort.Slice(r, func(i, j int) bool { return r[i].Flat > r[j].Flat })
	if len(r) > limit {
		r = r[:limit]
	}
	return r
}

func funcOf(loc *profile.Location) string {
	if len(loc.Line) == 0 || loc.Line[0].Function == nil {
		return ""
	}
	return loc.Line[0].Function.Name
}

func lineOf(loc *profile.Location) string {
	if len(loc.Line) == 0 || loc.Line[0].Function == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", loc.Line[0].Function.Filename, loc.Line[0].Line)
}

// ------------------------------------------------------------------ 落盘

func dumpDir() string {
	if Config.Dump != "" {
		return Config.Dump
	}
	return filepath.Join(os.TempDir(), "gomcp-pprof")
}

func dump(name string, raw []byte) (string, error) {
	dir := dumpDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	file := filepath.Join(dir, fmt.Sprintf("%s-%s.pb.gz", name, time.Now().Format("20060102-150405")))
	if err := os.WriteFile(file, raw, 0644); err != nil {
		return "", err
	}
	return file, nil
}

func load(file string) (*profile.Profile, int, error) {
	if file == "" {
		return nil, 0, fmt.Errorf("profile 文件路径不能为空")
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()
	p, err := profile.Parse(f)
	if err != nil {
		return nil, 0, err
	}
	return p, defaultSampleIndex(p), nil
}
