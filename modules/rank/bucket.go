package rank

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/logger"
)

func NewBucket(name string, zMax, zScore int64, zType SortType, handle Handle) *Bucket {
	return &Bucket{zName: name, zMax: zMax, zScore: zScore, zType: zType, handle: handle}
}

type Bucket struct {
	*Statement
	zMax    int64        //排行榜人数限制
	zType   SortType     //排序方式
	zName   string       //排行榜名称
	zScore  int64        //排行榜最低分数限制
	zMutex  sync.Mutex   //切换界数时使用
	handle  Handle       //获取当前期数和过期时间
	submits []*Statement //当前待结算
}

func (this *Bucket) start() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	_, _ = this.Cycle()
	// 检查Redis hash表中是否有未结算的排行榜
	this.checkUnsettledRanks()
	if this.zMax > 0 && this.Statement != nil {
		this.mayKeeper(this.Statement)
	}
	return
}

func (this *Bucket) RedisRankKey(circle int64) string {
	return fmt.Sprintf("%v-rk-%v-%v-%v", Options.ShareId, Options.ServerId, this.zName, circle)
}

func (this *Bucket) heartbeat() {
	defer func() {
		if e := recover(); e != nil {
			logger.Error(e)
		}
	}()
	if h, ok := this.handle.(HandleHeartbeat); ok {
		h.Heartbeat(this, this.Statement.zCycle)
	}

	_, _ = this.Cycle()
	if this.zMax > 0 {
		this.mayKeeper(this.Statement)
	}
	//结算
	if len(this.submits) > 0 {
		this.zMutex.Lock()
		stmt := this.submits[0]
		this.submits = this.submits[1:]
		this.zMutex.Unlock()
		this.maySubmit(stmt)
	}
}

func (this *Bucket) Writable(cycle int64) (r bool) {
	r = true
	_, e := this.handle.Expire(cycle)
	if truce := this.handle.Truce(); truce > 0 {
		if time.Now().Unix() >= e-truce {
			r = false
		}
	}
	return
}

// Cycle 获取当前第几期
func (this *Bucket) Cycle(skip ...int64) (cycle int64, writable bool) {
	n := int64(0)
	if len(skip) > 0 {
		n = skip[0]
	}
	cycle = this.handle.Cycle(n)
	writable = true
	//初始化 或 换届
	if this.Statement == nil || this.Statement.zCycle != cycle {
		this.changeCycle(cycle)
		return
	}
	//休战
	if truce := this.handle.Truce(); truce > 0 {
		now := time.Now().Unix()
		if now >= this.Statement.zExpire-truce {
			writable = false
		}
	}
	return
}

func (this *Bucket) Expire(cycle int64) (start, expire int64) {
	return this.handle.Expire(cycle)
}

func (this *Bucket) ZAdd(cycle int64, uid string, score int64) (err error) {
	v, writable := this.Cycle()
	if !writable {
		return nil //休战期不更新
	}
	if cycle == 0 {
		cycle = v
	} else if cycle != v {
		return nil //过期不更新
	}
	stmt := this.Statement
	if stmt.zCycle != cycle {
		return nil ///过期不更新
	}
	if !this.isMax(stmt, score) || !this.isScore(score) {
		return nil
	}
	return this.save(stmt, uid, score)
}

func (this *Bucket) ZRem(cycle int64, uid string) (err error) {
	key := this.RedisRankKey(cycle)
	return Redis.ZRem(context.Background(), key, uid).Err()
}

// ZCard 当前REDIS中的记录数
func (this *Bucket) ZCard(cycle int64) (n int64, err error) {
	key := this.RedisRankKey(cycle)
	if n, err = Redis.ZCard(context.Background(), key).Result(); err != nil {
		return
	}

	if this.zMax > 0 && n > this.zMax {
		n = this.zMax
	}
	return
}

// ZRank 返回个人名次
func (this *Bucket) ZRank(cycle int64, uid string, withScore bool) (r *Player, err error) {
	if cycle == 0 {
		cycle, _ = this.Cycle()
	}
	r = &Player{Uid: uid, Rank: -1}
	k := this.RedisRankKey(cycle)
	if this.zType == SortTypeDesc {
		r.Rank, err = Redis.ZRevRank(context.Background(), k, uid).Result()
	} else {
		r.Rank, err = Redis.ZRank(context.Background(), k, uid).Result()
	}
	if errors.Is(err, redis.Nil) {
		r.Rank = -1
		err = nil
	}
	if !withScore || r.Rank < 0 {
		return
	}
	var score float64
	if score, err = Redis.ZScore(context.Background(), k, uid).Result(); err != nil {
		return
	}
	r.Score = int64(math.Floor(score))
	return
}

// ZRange 区间信息
func (this *Bucket) ZRange(cycle int64, s, e int64) (r []*Player, err error) {
	k := this.RedisRankKey(cycle)
	var z []redis.Z
	if this.zType == SortTypeDesc {
		z, err = Redis.ZRevRangeWithScores(context.Background(), k, s, e).Result()
	} else {
		z, err = Redis.ZRangeWithScores(context.Background(), k, s, e).Result()
	}
	if err != nil {
		return
	}
	r = make([]*Player, 0, len(z))
	for i, v := range z {
		r = append(r, &Player{Score: int64(math.Floor(v.Score)), Uid: v.Member.(string), Rank: int64(i) + s})
	}
	return
}

// ZPlayer 根据名次获取玩家信息
// 注意,排名不存在时 返回 nil
func (this *Bucket) ZPlayer(cycle int64, rank int64) (r *Player, err error) {
	if rank < 0 {
		return nil, nil
	}
	var rs []*Player
	if rs, err = this.ZRange(cycle, rank, rank); err != nil {
		return
	}
	if len(rs) > 0 {
		r = rs[0]
	}
	return
}

// ZPage 排行榜列表
func (this *Bucket) ZPage(cycle int64, paging *cosmo.Paging) error {
	paging.Init(100)
	s := (paging.Page - 1) * paging.Size
	e := s + paging.Size - 1
	if cycle == 0 {
		cycle, _ = this.Cycle()
	}
	rank, err := this.ZRange(cycle, int64(s), int64(e))
	if err != nil {
		return err
	}
	paging.Rows = rank
	if paging.Total == 0 {
		var n int64
		if n, err = this.ZCard(cycle); err != nil {
			n = this.zMax
		}
		paging.Result(int(n))
	}
	return nil
}

// Range 遍历排行,用于结算发奖
func (this *Bucket) Range(cycle int64, handle func(player *Player) error) error {
	if cycle == 0 {
		cycle, _ = this.Cycle()
	}
	paging := &cosmo.Paging{}
	paging.Init(1000)
	n, err := this.ZCard(cycle)
	if err != nil {
		return err
	}
	if n > this.zMax {
		n = this.zMax
	}
	paging.Result(int(n))
	// 循环遍历每一页，从第1页到最后一页
	for paging.Page <= paging.Total {
		s := (paging.Page - 1) * paging.Size
		e := s + paging.Size - 1
		var rank []*Player
		if rank, err = this.ZRange(cycle, int64(s), int64(e)); err != nil {
			return err
		}
		for _, player := range rank {
			if err = handle(player); err != nil {
				return err
			}
		}
		paging.Page++
	}
	return nil
}

// Remove 删除,会保留最近3期,如果无过期时间立即删除
//
// delay 延时删除  <=0  立即删除
func (this *Bucket) Remove(cycle, delay int64) (err error) {
	key := this.RedisRankKey(cycle)
	if delay <= 0 {
		return Redis.Del(context.Background(), key).Err()
	}
	return Redis.Expire(context.Background(), key, time.Second*time.Duration(delay)).Err()
}

func (this *Bucket) save(stmt *Statement, uid string, score int64) (err error) {
	z := &redis.Z{Member: uid}
	z.Score = stmt.formatScore(this, score)
	key := this.RedisRankKey(stmt.zCycle)
	return Redis.ZAdd(context.Background(), key, z).Err()
}

// isMax 否满足入围名次
func (this *Bucket) isMax(stmt *Statement, v int64) bool {
	if this.zMax == 0 || stmt.zKeeper == 0 {
		return true
	}
	if this.zType == SortTypeDesc {
		return v > stmt.zKeeper
	} else {
		return v < stmt.zKeeper
	}
}

// isScore 是否满足入围分数
func (this *Bucket) isScore(v int64) bool {
	if this.zScore == 0 {
		return true
	}
	if this.zType == SortTypeDesc {
		return v >= this.zScore
	}
	return v < this.zScore
}

// mayKeeper 清理固定排行榜名次以外的数据
func (this *Bucket) mayKeeper(stmt *Statement) {
	if this.zMax == 0 {
		return
	}
	key := this.RedisRankKey(stmt.zCycle)
	v, err := Redis.ZCard(context.Background(), key).Result()
	if err != nil || v <= this.zMax+OverflowThreshold {
		return
	}
	p, err := this.ZPlayer(stmt.zCycle, this.zMax-1)
	if err != nil || p == nil {
		return
	}
	stmt.zKeeper = p.Score

	//移除有序集合中指定排名区间内的所有成员
	var start, stop int64
	if this.zType == SortTypeAsc {
		start = this.zMax
		stop = -1
	} else {
		start = 0
		stop = v - this.zMax - 1
	}
	Redis.ZRemRangeByRank(context.Background(), key, start, stop)
}
