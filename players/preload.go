package players

import (
	"fmt"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/players/player"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Preload
type preload interface {
	Record() int //需要预加载的实际条数
	Handle(page, size int, callback func(uid string, name string))
}

var Preload preload

// loading 初始加载用户到内存
func loading() (err error) {
	if Preload == nil {
		logger.Alert("未配置预加载接口(players.Preload),用户预加载功能未启用")
		return
	}
	record := Preload.Record()
	if record == 0 {
		return
	}
	if Options.MemoryInstall > 0 && record > Options.MemoryInstall {
		record = Options.MemoryInstall
	}

	size := cosmo.DefaultPageSize
	total := record / size
	if record%size > 0 {
		total += 1
	}
	logger.Trace("开始预加载数据,累计:%d条 共%d页", record, total)

	wg := &sync.WaitGroup{}
	progress := NewProgress(int32(record))
	progress.Wait(wg)
	for i := 1; i <= total; i++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			var pn int
			Preload.Handle(page, size, func(uid string, name string) {
				pn += 1
				progress.Add(1)
				p := player.New(uid)
				if e := p.Loading(true); e == nil {
					ps.Store(uid, p)
					p.KeepAlive(times.Now().Unix())
				}
			})
			//不满一页的
			if pn < size {
				progress.Add(int32(size - pn))
			}
		}(i)
	}
	wg.Wait()
	progress.Done()
	return
}

func NewProgress(total int32) Progress {
	return Progress{total: total, block: "="}
}

type Progress struct {
	total int32
	value int32
	block string
	done  bool
}

func (this *Progress) Wait(wg *sync.WaitGroup) {
	go func() {
		wg.Add(1)
		defer wg.Done()
		t := time.NewTicker(time.Millisecond * 10)
		defer t.Stop()
		for !this.done {
			<-t.C
			this.Printf()
		}
	}()
}

func (this *Progress) Add(v int32) {
	atomic.AddInt32(&this.value, v)
}
func (this *Progress) Done() {
	atomic.StoreInt32(&this.value, this.total)
}

func (this *Progress) Printf() {
	s, n := this.createProgressString()
	fmt.Printf("\r[%-50s] %d%%", s, n)

	if this.value >= this.total {
		this.done = true
		fmt.Println()
	}
}

func (this *Progress) createProgressString() (string, int32) {
	percent := float64(this.value) / float64(this.total)
	if percent > 1 {
		percent = 1
	}
	pn := int32(percent * 100)
	numBlocks := int(percent * 50) // 假设进度条长度为50
	var s string

	if numBlocks > 0 {
		s = strings.Repeat(this.block, numBlocks)
	}

	if n := 50 - numBlocks; n > 0 {
		s += strings.Repeat(" ", 50-numBlocks)
	}

	return s, pn
}
