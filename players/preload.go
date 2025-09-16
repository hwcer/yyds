package players

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hwcer/cosmo"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/players/player"
)

type preloadPlayerDecode struct {
	Id string `bson:"_id"`
}

// loading 初始加载用户到内存
func loading() (err error) {
	if Options.Preload == nil {
		logger.Alert("未配置预加载接口(Options.Preload)用户预加载功能未启用")
		return
	}

	var record int64
	tx := Options.Preload.TX()

	if err = tx.Count(&record).Error; err != nil {
		return
	}
	if record == 0 {
		return
	}
	if limit := Options.Preload.Limit(); limit > 0 && record > limit {
		record = limit
	}
	logger.Trace("开始预加载数据,累计:%d条", record)
	progress := newProgress(record)
	tx = tx.Select("_id").Limit(int(record))
	tx = tx.Range(func(cursor cosmo.Cursor) bool {
		v := &preloadPlayerDecode{}
		if e := cursor.Decode(v); e == nil {
			progress.c <- v.Id
			return true
		} else {
			tx.Errorf(err)
			return false
		}
	})
	if tx.Error != nil {
		return tx.Error
	}
	progress.Wait()

	return
}

func newProgress(total int64) *Progress {
	r := &Progress{total: total, block: "="}
	r.c = make(chan string, 1000)
	r.wg = &sync.WaitGroup{}
	r.done = make(chan struct{})
	for i := 0; i < 10; i++ {
		r.wg.Add(1)
		go r.loading()
	}
	return r
}

type Progress struct {
	total int64
	value int64
	block string
	done  chan struct{}
	wg    *sync.WaitGroup
	c     chan string
}

func (this *Progress) loading() {
	defer this.wg.Done()
	for {
		select {
		case <-this.done:
			return
		case uid, ok := <-this.c:
			if ok {
				this.player(uid)
			} else {
				return
			}
		}
	}
}
func (this *Progress) player(uid string) {
	defer func() {
		_ = recover()
		this.Add(1)
	}()
	p := player.New(uid)
	if e := p.Loading(true); e == nil {
		ps.Store(uid, p)
		p.KeepAlive(time.Now().Unix())
	}
}

func (this *Progress) Wait() {
	this.wg.Add(1)
	go func() {
		defer this.wg.Done()
		t := time.NewTicker(time.Millisecond * 100)
		defer t.Stop()
		for {
			select {
			case <-this.done:
				return
			case <-t.C:
				this.Printf()
			}
		}
	}()
	this.wg.Wait()
}

func (this *Progress) Add(v int64) {
	atomic.AddInt64(&this.value, v)
}

func (this *Progress) Printf() {
	s, n := this.createProgressString()
	fmt.Printf("\r[%-50s] %d%%", s, n)
	if this.value >= this.total && len(this.c) == 0 {
		fmt.Println()
		close(this.done)
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
