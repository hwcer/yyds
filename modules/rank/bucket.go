package rank

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/logger"
)

func NewBucket(name any, zKey string, zMax, zScore int64, zType SortType, handle Handle, opts ...Option) *Bucket {
	b := &Bucket{zName: name, zKey: zKey, zMax: zMax, zScore: zScore, zType: zType, handle: handle}
	b.zTiebreak = true //默认启用同分tiebreak,由 WithoutTiebreak 关闭
	for _, opt := range opts {
		if opt != nil {
			opt(b)
		}
	}
	return b
}

type Bucket struct {
	zMax      int64                     //排行榜人数限制
	zKey      string                    //排行榜名称转换后的确定字符串,用于生成REDIS KEY
	zType     SortType                  //排序方式
	zName     any                       //排行榜名称,仅支持字符串和数字
	zScore    int64                     //排行榜最低分数限制
	zTiebreak bool                      //是否把达成时间编码进score小数位做同分tiebreak,见 WithoutTiebreak
	zStmt     atomic.Pointer[Statement] //当前届,换届时整体替换,业务协程无锁读取
	zMutex    sync.Mutex                //保护 submits 以及换届过程
	handle    Handle                    //获取当前期数和过期时间
	submits   []*Statement              //当前待结算,只由心跳协程消费
}

// Name 排行榜名称(原始值)
func (this *Bucket) Name() any {
	return this.zName
}

// statement 当前届,未初始化时返回nil
func (this *Bucket) statement() *Statement {
	return this.zStmt.Load()
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
	if stmt := this.statement(); this.zMax > 0 && stmt != nil {
		this.mayKeeper(stmt)
	}
	return
}

func (this *Bucket) RedisRankKey(circle int64) string {
	return fmt.Sprintf("%v-rk-%v-%v-%v", Options.ShareId, Options.ServerId, this.zKey, circle)
}

func (this *Bucket) heartbeat() {
	defer func() {
		if e := recover(); e != nil {
			logger.Error(e)
		}
	}()
	//先换届,保证 HandleHeartbeat 拿到的是最新期数
	cycle, _ := this.Cycle()
	if h, ok := this.handle.(HandleHeartbeat); ok {
		h.Heartbeat(this, cycle)
	}
	if stmt := this.statement(); this.zMax > 0 && stmt != nil {
		this.mayKeeper(stmt)
	}
	//结算,每次心跳只处理一个,避免阻塞心跳协程
	this.zMutex.Lock()
	var stmt *Statement
	if len(this.submits) > 0 {
		stmt = this.submits[0]
		this.submits = this.submits[1:]
	}
	this.zMutex.Unlock()
	if stmt != nil {
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
	//查询其他期数时只返回期数,不得触发换届,否则会把当前届回退成历史届
	if n != 0 {
		return
	}
	//初始化 或 换届
	stmt := this.statement()
	if stmt == nil || stmt.zCycle != cycle {
		this.changeCycle(cycle)
		return
	}
	//休战
	if truce := this.handle.Truce(); truce > 0 {
		if time.Now().Unix() >= stmt.zExpire-truce {
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
	stmt := this.statement()
	if stmt == nil || stmt.zCycle != cycle {
		return nil ///过期不更新
	}
	if !this.isMax(stmt, score) || !this.isScore(score) {
		return nil
	}
	return this.save(stmt, uid, score)
}

// swapScript 原子对调两名成员的分数
//
// 必须整体在 Lua 内完成:若在 Go 侧先 ZSCORE 读出来比大小、再发两条 ZADD,那个"读-判-写"
// 不是原子的——并发下目标的名次可能在读与写之间被第三方换走,判定即失效;
// 两条 ZADD 之间还存在中间态,此刻别的请求会读到两人同分。
//
// 返回 ZSCORE 的原始字符串而非 number:REDIS 的 Lua 返回 number 时会截断小数,
// 启用了 tiebreak 的榜其 score 带小数位,直接返回 number 会丢精度。
var swapScript = redis.NewScript(`
local a = redis.call('ZSCORE', KEYS[1], ARGV[1])
local b = redis.call('ZSCORE', KEYS[1], ARGV[2])
if (not a) or (not b) then return {'missing'} end
if ARGV[3] == '1' then
  local ahead
  if ARGV[4] == '1' then
    ahead = tonumber(b) > tonumber(a)
  else
    ahead = tonumber(b) < tonumber(a)
  end
  if not ahead then return {'cond'} end
end
redis.call('ZADD', KEYS[1], b, ARGV[1])
redis.call('ZADD', KEYS[1], a, ARGV[2])
return {'ok', a, b}
`)

// ZSwap 原子对调 a 与 b 的分数,返回各自对调【前】的原始分数
//
// 适用于"名次即资源"的榜:挑战成功后攻守双方交换席位。cond 见 SwapCond。
// 不做入围分数(isScore)与名次(isMax)检查——双方本就都在榜上,与"够不够格上榜"无关。
//
// 错误:ErrTruce(休战期) / ErrSwapMemberMissing(有人不在榜) / ErrSwapCondition(条件不满足)
func (this *Bucket) ZSwap(cycle int64, a, b string, cond SwapCond) (scoreA, scoreB int64, err error) {
	if a == "" || b == "" || a == b {
		return 0, 0, values.Error("rank: ZSwap invalid uid")
	}
	v, writable := this.Cycle()
	if !writable {
		return 0, 0, ErrTruce
	}
	if cycle == 0 {
		cycle = v
	} else if cycle != v {
		return 0, 0, ErrSwapMemberMissing //非当前届不可交换
	}
	stmt := this.statement()
	if stmt == nil || stmt.zCycle != cycle {
		return 0, 0, ErrSwapMemberMissing
	}
	desc := "0"
	if this.zType == SortTypeDesc {
		desc = "1"
	}
	key := this.RedisRankKey(cycle)
	res, e := swapScript.Run(context.Background(), Redis, []string{key}, a, b, fmt.Sprintf("%d", cond), desc).StringSlice()
	if e != nil {
		return 0, 0, e
	}
	if len(res) == 0 {
		return 0, 0, values.Error("rank: ZSwap empty reply")
	}
	switch res[0] {
	case "missing":
		return 0, 0, ErrSwapMemberMissing
	case "cond":
		return 0, 0, ErrSwapCondition
	case "ok":
		if len(res) < 3 {
			return 0, 0, values.Error("rank: ZSwap bad reply")
		}
		fa, _ := strconv.ParseFloat(res[1], 64)
		fb, _ := strconv.ParseFloat(res[2], 64)
		return parseScore(fa), parseScore(fb), nil
	}
	return 0, 0, values.Errorf(0, "rank: ZSwap unknown reply:%v", res[0])
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
	r.Score = parseScore(score)
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
		r = append(r, &Player{Score: parseScore(v.Score), Uid: v.Member.(string), Rank: int64(i) + s})
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
	//ZCard 内部已按 zMax 截断,此处不能再用 zMax 兜底,否则 zMax==0(不限人数)时会得到0
	n, err := this.ZCard(cycle)
	if err != nil {
		return err
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

// Remove 删除指定届的排行榜数据
//
// delay 延时删除(s),<=0 立即删除
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
//
// 这里只是尽力而为的前置过滤,用于省掉注定会被裁掉的写入;
// 真正把人数压到 zMax 的是 mayKeeper 里的 ZRemRangeByRank。
// 所以守门员分数恰好为0时退化成不过滤是可以接受的,不值得为此再加一个标记位
func (this *Bucket) isMax(stmt *Statement, v int64) bool {
	keeper := stmt.zKeeper.Load()
	if this.zMax == 0 || keeper == 0 {
		return true
	}
	if this.zType == SortTypeDesc {
		return v > keeper
	}
	return v < keeper
}

// isScore 是否满足入围分数,边界值本身也算入围
func (this *Bucket) isScore(v int64) bool {
	if this.zScore == ScoreUnlimited {
		return true
	}
	if this.zType == SortTypeDesc {
		return v >= this.zScore
	}
	return v <= this.zScore
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
	stmt.zKeeper.Store(p.Score)

	//移除有序集合中指定排名区间内的所有成员
	var start, stop int64
	if this.zType == SortTypeAsc {
		start = this.zMax
		stop = -1
	} else {
		start = 0
		stop = v - this.zMax - 1
	}
	if err := Redis.ZRemRangeByRank(context.Background(), key, start, stop).Err(); err != nil {
		logger.Error("ZRemRangeByRank error, key:%v, err:%v", key, err)
	}
}
