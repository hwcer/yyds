package players

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hwcer/cosmo"
	"github.com/hwcer/logger"
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
		if e := cursor.Decode(v); e != nil {
			logger.Alert("preload error: %v", e)
			progress.Add(1)
		} else {
			progress.c <- v.Id
		}

		return true
	})
	close(progress.c)
	if tx.Error != nil {
		err = tx.Error
	}
	//🔴 必须无条件 Wait:done 的关闭原先只由 Printf 里「value >= total && len(c)==0」触发,
	//而 Count 与 Range 是两次查询,期间有玩家被删就会出现 rows < record → value 永远到不了
	//total → done 不关 → printer 协程不退 → **Start 永久挂死**。改为 workers(随 c 关闭退出)
	//全部结束后由这里兜底关闭 done,计数不符只影响进度条显示,不影响启动。
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
	once  sync.Once
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
	p := NewPlayer(uid, false)
	if e := p.Loading(false); e == nil {
		manage.Store(p.Key(), p)
		p.KeepAlive(time.Now().Unix())
	}
}

func (this *Progress) Wait() {
	//printer 不计入 wg:它等 done 退出,而 done 要等 workers(wg)全部结束后才由这里关闭,
	//算进同一个 WaitGroup 就是互相等待的死锁
	go func() {
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
	//workers 已全部退出(通道关闭),无论计数是否对得上都收掉 printer,见 loading 里的说明
	this.once.Do(func() { close(this.done) })
}

func (this *Progress) Add(v int64) {
	atomic.AddInt64(&this.value, v)
}

func (this *Progress) Printf() {
	s, n := this.createProgressString()
	fmt.Printf("\r[%-50s] %d%%", s, n)
	if this.value >= this.total && len(this.c) == 0 {
		fmt.Println()
		this.once.Do(func() { close(this.done) })
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
