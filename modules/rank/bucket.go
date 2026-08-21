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
	//休战判定是可选能力,在此一次性断言并缓存:它会被每次读写路径调用,
	//每次都做一遍类型断言纯属浪费。
	b.truce, _ = handle.(HandleTruce)
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
	truce     HandleTruce               //可选:休战判定,handle 未实现该接口时为 nil(即永不休战)
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

// Writable 指定届当前是否可写(不在休战期)
func (this *Bucket) Writable(cycle int64) bool {
	return !this.truced(cycle)
}

// truced 是否处于休战期。handle 未实现 HandleTruce 即永不休战。
func (this *Bucket) truced(cycle int64) bool {
	return this.truce != nil && this.truce.Truce(cycle)
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
	writable = !this.truced(cycle)
	return
}

func (this *Bucket) Expire(cycle int64) (start, expire int64) {
	return this.handle.Expire(cycle)
}

func (this *Bucket) ZAdd(cycle int64, uid string, score int64) (err error) {
	v, writable := this.Cycle()
	if !writable {
		//🔴 休战期【显式报错】,不再静默返回 nil。
		//旧行为是个长期的坑:玩家扣了票、发了奖、分数却没写进去,
		//而调用方拿到 nil 以为成功,日志里什么都看不到。
		//ZSwap / ZAdds 从一开始就没沿用那个静默语义,这里与它们对齐。
		return ErrTruce
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

// zAddsChunk 单条 ZADD 携带的成员上限。
//
// REDIS 是单线程的,一条命令的执行时间就是所有其他客户端的等待时间;
// 且一条命令要整体驻留内存。分片既压住单条命令的体量,又保住批量的收益
// ——5000 个成员分 5 条,与逐条发 5000 次相比仍是三个数量级的差距。
const zAddsChunk = 1000

// ZAdds 批量写分,供建榜 / 换届重建这类成百上千条的一次性写入。
//
// 逐条 ZAdd 时每条都是一次 RTT:实测跨网段写 5000 条约 3.6 秒(约 0.72ms/条,
// 基本是纯网络往返)。批量把它压成 ceil(n/zAddsChunk) 次往返。
//
// 返回实际写入的条数:入围分数(zScore)与守门员(zKeeper)过滤掉的不计。
//
// 🔴 与 ZAdd 的语义差异:休战期 / 届已过期时,ZAdd 是【静默返回 nil】,本接口【显式报错】。
// 不是不一致,是场景不同——ZAdd 单条写分失败无伤大雅(下次打分再写),
// 而批量写的调用方多半在建榜:静默不写会留下一张空榜,调用方却以为填好了,
// 之后所有"取我上方/下方的对手"都取空。ZSwap 当初也是为同一个理由没有沿用静默语义。
func (this *Bucket) ZAdds(cycle int64, members []Member) (n int, err error) {
	if len(members) == 0 {
		return 0, nil
	}
	v, writable := this.Cycle()
	if !writable {
		return 0, ErrTruce
	}
	if cycle == 0 {
		cycle = v
	} else if cycle != v {
		return 0, ErrCycleExpired
	}
	stmt := this.statement()
	if stmt == nil || stmt.zCycle != cycle {
		return 0, ErrCycleExpired
	}
	key := this.RedisRankKey(stmt.zCycle)
	buf := make([]*redis.Z, 0, zAddsChunk)
	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		if e := client.ZAdd(context.Background(), key, buf...).Err(); e != nil {
			return e
		}
		n += len(buf)
		buf = buf[:0]
		return nil
	}
	for i := range members {
		m := &members[i]
		if m.Uid == "" {
			continue
		}
		if !this.isMax(stmt, m.Score) || !this.isScore(m.Score) {
			continue
		}
		buf = append(buf, &redis.Z{Member: m.Uid, Score: stmt.formatScore(this, m.Score)})
		if len(buf) >= zAddsChunk {
			if err = flush(); err != nil {
				return
			}
		}
	}
	err = flush()
	return
}

// swapScript 原子对调两名成员的分数
//
// 必须整体在 Lua 内完成:若在 Go 侧先 ZSCORE 读出来比大小、再发两条 ZADD,那个"读-判-写"
// 不是原子的——并发下目标的名次可能在读与写之间被第三方换走,判定即失效;
// 两条 ZADD 之间还存在中间态,此刻别的请求会读到两人同分。
//
// 返回 ZSCORE 的原始字符串而非 number:REDIS 的 Lua 返回 number 时会截断小数,
// 启用了 tiebreak 的榜其 score 带小数位,直接返回 number 会丢精度。
//
// 🔴 两处与"排序"有关的坑:
//
//	1.【同分直接拒绝】ZSET 同分时按 member 字典序排,此时互换分数是 no-op ——
//	  名次纹丝不动,却会让调用方以为换成功了(静默失败)。故 score 相等一律返回 equal。
//	2.【条件判定用 ZRANK 而不是比分数】同分时分数比较反映不了真实名次(见上)。
//	  改用 ZRANK/ZREVRANK 取真实名次比较,rank 恒为"越小越靠前",顺带把升序/降序两种
//	  榜的方向差异收敛掉,不必在脚本里按 SortType 反转比较符。
var swapScript = redis.NewScript(`
local a = redis.call('ZSCORE', KEYS[1], ARGV[1])
local b = redis.call('ZSCORE', KEYS[1], ARGV[2])
if (not a) or (not b) then return {'missing'} end
if tonumber(a) == tonumber(b) then return {'equal'} end
if ARGV[3] == '1' then
  local ra, rb
  if ARGV[4] == '1' then
    ra = redis.call('ZREVRANK', KEYS[1], ARGV[1])
    rb = redis.call('ZREVRANK', KEYS[1], ARGV[2])
  else
    ra = redis.call('ZRANK', KEYS[1], ARGV[1])
    rb = redis.call('ZRANK', KEYS[1], ARGV[2])
  end
  if rb >= ra then return {'cond'} end
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
	res, e := swapScript.Run(context.Background(), client, []string{key}, a, b, fmt.Sprintf("%d", cond), desc).StringSlice()
	if e != nil {
		return 0, 0, e
	}
	if len(res) == 0 {
		return 0, 0, values.Error("rank: ZSwap empty reply")
	}
	switch res[0] {
	case "missing":
		return 0, 0, ErrSwapMemberMissing
	case "equal":
		return 0, 0, ErrSwapEqualScore
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

// ZPreset 预置某一届的完整榜单——在该届【开始之前】把数据写好
//
// 场景:换届结算要在上一届的最后时段完成(例如每天最后半小时冻结榜、跑结算),
// 那一刻新一届还没开始,ZAdd/ZAdds 会因为「不是当前届」返回 ErrCycleExpired,
// 且冻结期本身还会被休战检查拦成 ErrTruce。ZPreset 是唯一能穿过这两道的写入口。
//
// 🔴 **只能写未来的届**。写当前届会覆盖正在用的榜(玩家名次凭空重排),
// 写过去的届会污染已结算的历史数据,两者一律拒绝(ErrPresetNotFuture)。
//
// 🔴 **写临时键再 RENAME,不直接写目标键**。整批 5000 条要分多次 ZADD,
// 中途若跨过了届的边界(目标届变成当前届),直接写就会留下一个只填了一半的活榜——
// 而它非空,业务侧的「榜空即补」兜底不会触发,这一届就永久缺人。
// 改成写临时键 + 最后原子 RENAME:中途放弃时目标键仍是空的,兜底照常生效。
//
// 传空成员列表视为调用方的错误:预置一个空榜没有任何意义,只会让 RENAME 失败。
func (this *Bucket) ZPreset(cycle int64, members []Member) (n int, err error) {
	if cycle <= 0 {
		return 0, values.Error("rank: ZPreset invalid cycle")
	}
	if len(members) == 0 {
		return 0, values.Error("rank: ZPreset empty members")
	}
	cur, _ := this.Cycle() //不看 writable:预置本来就是要在休战窗口里做的事
	if cycle <= cur {
		return 0, ErrPresetNotFuture
	}
	//目标届还没开始,拿不到它的 Statement,按业务回调算出的起止时间造一个。
	//tiebreak 的小数位依赖「本届已过时间占比」,届未开始时 elapsed 为负、被钳到 0,
	//即全体同分先到先得的初值——正是预置该有的语义。
	zt, ze := this.handle.Expire(cycle)
	stmt := NewStatement(cycle, zt, ze)

	dst := this.RedisRankKey(cycle)
	tmp := dst + "-preset"
	ctx := context.Background()
	if err = client.Del(ctx, tmp).Err(); err != nil {
		return 0, err
	}
	buf := make([]*redis.Z, 0, zAddsChunk)
	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		if e := client.ZAdd(ctx, tmp, buf...).Err(); e != nil {
			return e
		}
		n += len(buf)
		buf = buf[:0]
		return nil
	}
	for i := range members {
		m := &members[i]
		if m.Uid == "" || !this.isScore(m.Score) {
			continue
		}
		//不做 isMax 检查:那是按当前届的 zKeeper(末位分数)裁溢出的,
		//目标届还没开始、根本没有 keeper,拿当前届的去卡新榜是张冠李戴
		buf = append(buf, &redis.Z{Member: m.Uid, Score: stmt.formatScore(this, m.Score)})
		if len(buf) >= zAddsChunk {
			if err = flush(); err != nil {
				_ = client.Del(ctx, tmp).Err()
				return 0, err
			}
		}
	}
	if err = flush(); err != nil {
		_ = client.Del(ctx, tmp).Err()
		return 0, err
	}
	if n == 0 {
		_ = client.Del(ctx, tmp).Err()
		return 0, values.Error("rank: ZPreset all members filtered out")
	}
	//落地前再确认一次:写这一批的工夫里届可能已经推进,那就整批作废,
	//让目标键保持空,由业务侧的兜底重建接手
	if cur, _ = this.Cycle(); cycle <= cur {
		_ = client.Del(ctx, tmp).Err()
		return 0, ErrPresetNotFuture
	}
	if err = client.Rename(ctx, tmp, dst).Err(); err != nil {
		_ = client.Del(ctx, tmp).Err()
		return 0, err
	}
	return n, nil
}

// takeoverScript 榜外成员顶替榜内成员:a 接手 b 的分数,b 被移出榜
//
// 与 swapScript 同理必须整体在 Lua 内完成:ZREM 与 ZADD 之间存在中间态,
// 此刻别的请求会读到"这一席空着",并发下两个榜外玩家可能同时顶上同一席。
//
// 🔴 要求 a **不在**榜上。a 已在榜上时该用 ZSwap——那是双方互换,总数不变;
// 这里是 b 出榜、a 进榜,若 a 本就在榜,执行下去等于把 a 原来那一席凭空丢掉。
var takeoverScript = redis.NewScript(`
local a = redis.call('ZSCORE', KEYS[1], ARGV[1])
local b = redis.call('ZSCORE', KEYS[1], ARGV[2])
if not b then return {'missing'} end
if a then return {'exists'} end
redis.call('ZREM', KEYS[1], ARGV[2])
redis.call('ZADD', KEYS[1], b, ARGV[1])
return {'ok', b}
`)

// ZTakeover 榜外成员 a 顶替榜内成员 b,接手其分数(名次),b 转为榜外。返回接手到的分数
//
// 适用于"名次即资源"且**榜容量固定**的榜:榜外玩家打赢榜底对手后占下那一席,
// 榜内总数恒定不变。与 ZSwap 的分工:双方都在榜上用 ZSwap(互换),
// 攻方在榜外用 ZTakeover(顶替)。
//
// 同样不做入围分数(isScore)与名次(isMax)检查——顶替是"接手一个已存在的席位",
// 席位本身就在榜内,与"够不够格上榜"无关。
//
// 错误:ErrTruce(休战期) / ErrSwapMemberMissing(b 不在榜) / ErrTakeoverMemberExists(a 已在榜)
func (this *Bucket) ZTakeover(cycle int64, a, b string) (score int64, err error) {
	if a == "" || b == "" || a == b {
		return 0, values.Error("rank: ZTakeover invalid uid")
	}
	v, writable := this.Cycle()
	if !writable {
		return 0, ErrTruce
	}
	if cycle == 0 {
		cycle = v
	} else if cycle != v {
		return 0, ErrSwapMemberMissing //非当前届不可顶替
	}
	stmt := this.statement()
	if stmt == nil || stmt.zCycle != cycle {
		return 0, ErrSwapMemberMissing
	}
	key := this.RedisRankKey(cycle)
	res, e := takeoverScript.Run(context.Background(), client, []string{key}, a, b).StringSlice()
	if e != nil {
		return 0, e
	}
	if len(res) == 0 {
		return 0, values.Error("rank: ZTakeover empty reply")
	}
	switch res[0] {
	case "missing":
		return 0, ErrSwapMemberMissing
	case "exists":
		return 0, ErrTakeoverMemberExists
	case "ok":
		if len(res) < 2 {
			return 0, values.Error("rank: ZTakeover bad reply")
		}
		f, _ := strconv.ParseFloat(res[1], 64)
		return parseScore(f), nil
	}
	return 0, values.Errorf(0, "rank: ZTakeover unknown reply:%v", res[0])
}

func (this *Bucket) ZRem(cycle int64, uid string) (err error) {
	key := this.RedisRankKey(cycle)
	return client.ZRem(context.Background(), key, uid).Err()
}

// ZCard 当前REDIS中的记录数
func (this *Bucket) ZCard(cycle int64) (n int64, err error) {
	key := this.RedisRankKey(cycle)
	if n, err = client.ZCard(context.Background(), key).Result(); err != nil {
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
		r.Rank, err = client.ZRevRank(context.Background(), k, uid).Result()
	} else {
		r.Rank, err = client.ZRank(context.Background(), k, uid).Result()
	}
	if errors.Is(err, redis.Nil) {
		r.Rank = -1
		err = nil
	}
	//超出 zMax 的一律视为未上榜:溢出清理是延时的(要 ZCARD>zMax+OverflowThreshold 才触发,
	//且由 5s 心跳驱动),期间 REDIS 里真实存在 zMax 之外的成员。若如实返回,玩家会看到
	//"第 5200 名"这种榜容量之外的名次,与 ZCard 截断后的总数自相矛盾。
	if this.outOfRange(r.Rank) {
		r.Rank = -1
	}
	if !withScore || r.Rank < 0 {
		return
	}
	var score float64
	if score, err = client.ZScore(context.Background(), k, uid).Result(); err != nil {
		return
	}
	r.Score = parseScore(score)
	return
}

// ZRange 区间信息
//
// 区间自动截断到 zMax 之内:溢出清理是延时的,REDIS 里可能存在 zMax 之外的成员,
// 不截断会把它们当作榜内数据返回。zMax==0(不限人数)时不做限制。
func (this *Bucket) ZRange(cycle int64, s, e int64) (r []*Player, err error) {
	if this.zMax > 0 {
		if s >= this.zMax {
			return []*Player{}, nil //起点已在榜外
		}
		if e < 0 || e >= this.zMax {
			e = this.zMax - 1
		}
	}
	k := this.RedisRankKey(cycle)
	var z []redis.Z
	if this.zType == SortTypeDesc {
		z, err = client.ZRevRangeWithScores(context.Background(), k, s, e).Result()
	} else {
		z, err = client.ZRangeWithScores(context.Background(), k, s, e).Result()
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
		return client.Del(context.Background(), key).Err()
	}
	return client.Expire(context.Background(), key, time.Second*time.Duration(delay)).Err()
}

// outOfRange 名次是否已在榜容量之外(zMax==0 表示不限人数,恒为 false)
func (this *Bucket) outOfRange(rank int64) bool {
	return this.zMax > 0 && rank >= this.zMax
}

func (this *Bucket) save(stmt *Statement, uid string, score int64) (err error) {
	z := &redis.Z{Member: uid}
	z.Score = stmt.formatScore(this, score)
	key := this.RedisRankKey(stmt.zCycle)
	return client.ZAdd(context.Background(), key, z).Err()
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
	v, err := client.ZCard(context.Background(), key).Result()
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
	if err := client.ZRemRangeByRank(context.Background(), key, start, stop).Err(); err != nil {
		logger.Error("ZRemRangeByRank error, key:%v, err:%v", key, err)
	}
}
