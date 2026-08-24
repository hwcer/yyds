package rank

import (
	"errors"
	"math"
	"time"

	"github.com/hwcer/cosgo/redis"
)

type SortType int8

const (
	SortTypeAsc  SortType = -1
	SortTypeDesc SortType = 1
)

// 排行榜系统
const (
	Heartbeat         = 5   //心跳间隔(s)
	OverflowThreshold = 500 //排行榜人数超过预设值N个时触发清理
	//DefaultRetention 无法计算周期长度时,结算后数据的兜底保留时长
	DefaultRetention = 7 * 24 * time.Hour
)

// ScoreUnlimited 用于 Register 的 zScore 参数,表示不限制入围分数
//
// 注意不能用0表示不限制:0是合法的入围门槛(如降序榜只收非负分),
// 传0的含义是"分数必须>=0(降序)或<=0(升序)"
const ScoreUnlimited int64 = math.MinInt64

var Options = struct {
	ShareId   string
	ServerId  int32
	StartTime string //开服时间
}{
	StartTime: "2024-01-01 00:00:00+0800",
}

// client 排行榜专用的 Redis 连接。
//
// 🔴 **私有,只有 Start 能写**。它曾经是导出的 `Redis`,于是外部可以直接赋值——
// 而 Start 开头的 `if Redis != nil { return nil }` 本意是防重复启动,一旦有人
// 提前赋了值,Start 就整个空转:ShareId/ServerId 不设(REDIS key 丢掉分区前缀,
// 多服共用一个 Redis 时互相踩)、started 不置位(注册保护失效)、
// Master.start() 不执行(statement 没建、心跳没起、未结算届没检查)。
//
// 而且全程不报错——榜看着能读能写,只是届号、换届、结算全是坏的。
// 封死写入口之后,`client != nil` 才真正等价于「Start 调用过」。
var client *redis.Client

// Redis 取排行榜的 Redis 连接（只读）。
//
// 给需要直接操作 REDIS 的业务用（如自己的标记键）;**不要拿它去写榜数据**,
// 那样会绕开分数编码(formatScore)与休战判定,写出格式不一致的分数。
func Redis() *redis.Client {
	return client
}

// SwapCond ZSwap 的交换条件,在 Lua 内部重查后判定,保证"读-判-写"整体原子
//
// # 升序/降序的方向差异是怎么消掉的
//
// 判定比的是【名次】而不是分数:降序榜取 ZREVRANK、升序榜取 ZRANK,两条路出来的
// rank 恒为"越小越靠前",于是比较符只有 rb < ra 一个,不必按 SortType 翻转。
// 方向判断只发生在"取哪个 rank 函数"这一处——比翻转比较符稳:
// 以后改条件语义时,翻转符号那种写法极容易只改一半。
//
// ⚠ a 在榜外时【不参与】这个判定:榜外不在分数空间里,既没有分数也没有名次,
// 一律视为"排在所有人之后"。这对升序降序同时成立(它只跟"有没有席位"有关,
// 跟分数怎么映射到名次无关),故 SwapIfTargetAhead 在顶替路径恒放行。
type SwapCond int8

const (
	SwapAlways        SwapCond = 0 //无条件对调
	SwapIfTargetAhead SwapCond = 1 //仅当 b 比 a 更靠前时才对调
)

// SwapKind ZSwap 的结局:a 原本在不在榜上,决定了 b 的去向
//
// 调用方通常需要区分——顶替会把 b 挤出榜(要发邮件、清它的名次缓存),互换不会。
// 也正因为要区分,SwapResult 才不能用 ScoreA==0 表示"a 原本不在榜":0 是合法分数,
// 那样调用方分不出"a 原来 0 分"和"a 根本没上榜"。
type SwapKind int8

const (
	SwapKindSwap     SwapKind = 1 //双方都在榜:互换分数,b 退到 a 原来的名次
	SwapKindTakeover SwapKind = 2 //a 原在榜外:接手 b 的名次,b 出榜。榜内人数不变
)

// SwapResult ZSwap 成功时的结果
type SwapResult struct {
	Kind   SwapKind //见 SwapKind,区分 b 是"被换到后面"还是"被挤出榜"
	ScoreA int64    //a 交换【前】的分数。Kind==SwapKindTakeover 时无意义(a 本不在榜)
	ScoreB int64    //b 交换【前】的分数,即 a 接手到的分数。两种结局下都有效
}

// ZSwap 的错误,业务侧需按 errors.Is 区分处理
var (
	//ErrSwapMemberMissing 被挑战方 b 不在榜上(退榜/换届/从未入榜),没有席位可夺
	//
	//注意只约束 b:a 在不在榜都是合法输入,见 Bucket.ZSwap。
	ErrSwapMemberMissing = errors.New("rank: member not on the list")
	//ErrPresetNotEmpty 预置的目标届已有数据,拒绝覆盖(在用的榜 / 已结算的历史数据)
	ErrPresetNotEmpty = errors.New("rank: preset target already has data")
	//ErrSwapCondition 交换条件不满足,如要求目标更靠前但其名次已被他人换走
	ErrSwapCondition = errors.New("rank: swap condition not satisfied")
	//ErrSwapEqualScore 双方分数相同,交换是 no-op
	//
	//ZSET 同分时按 member 字典序排,互换相同的分数改变不了任何人的名次。
	//这种情况必须显式报错而非返回成功,否则调用方会以为交换生效了。
	ErrSwapEqualScore = errors.New("rank: equal score, swap would be a no-op")
	//ErrNotStarted 排行榜模块尚未启动(未配置 Redis / Start 未调用)
	ErrNotStarted = errors.New("rank: module not started")
	//ErrCycleExpired 目标届已不是当前届(换届/传错届号),批量写入拒绝执行
	ErrCycleExpired = errors.New("rank: cycle expired")
	//ErrTruce 休战期内不可写
	//
	//🔴 休战约束的是【改变名次】,不是【建立榜单】:
	//	受约束   ZAdd / ZSwap  —— 玩家驱动:写分、席位争夺
	//	不受约束 ZAdds / ZPreset —— 系统驱动:活榜批量写、从空建榜
	//
	//系统写榜本来就发生在休战窗口里(换届冻结→结算→填新一届、定时重算全服分数),
	//拦了它,那两个接口在自己的正经用途上永远失败。
	//
	//受约束的两条路径一律【显式报错】而非静默丢弃:静默失败是个长期的坑——
	//玩家扣了票、发了奖、分数却没写进去,而调用方拿到 nil 以为成功,日志里什么都看不到。
	ErrTruce = errors.New("rank: truce period, list is read-only")
)

// Option 注册排行榜时的可选项
type Option func(*Bucket)

// WithoutTiebreak 关闭同分 tiebreak,令 REDIS score 保持纯整数
//
// 默认 formatScore 会把"本届已过时间"编码进 score 的小数位,用于同分时"先达成者排前"。
// 当分数本身即唯一序号时(如名次席位榜:score 就是名次,天然不可能同分),这层编码纯属多余:
// 读侧必须处处记得 parseScore 还原,一旦有路径直接拿 ZSCORE 的浮点去比较就会出错;
// 若还存在绕开 formatScore 的写入路径(如 Lua 脚本直接 ZADD),同一个榜里会并存两种分数格式。
func WithoutTiebreak() Option {
	return func(b *Bucket) { b.zTiebreak = false }
}

type Handle interface {
	Cycle(skip int64) (cycle int64)           //返回本期标记,skip :0 当前，-1：上一届，1：下一届。。。
	Expire(cycle int64) (start, expire int64) //当前届排行榜的开始时间和结束时间,均为unix秒
	//Submit 结算,返回排行榜数据的删除时间(unix秒)
	//
	//返回值小于当前时间时(如返回0)使用 DefaultRetention 兜底
	Submit(b *Bucket, cycle int64) (expire int64, err error)
}

type HandleHeartbeat interface {
	Heartbeat(w *Bucket, cycle int64)
}

// HandleTruce 可选:由排行榜【自己】判断当前是否处于休战期(榜只读)。
//
// 不实现即永不休战。
//
// # 为什么从 Truce() int64 改成回调
//
// 旧签名返回「本届结束前 X 秒」,只能表达**一种固定形状**:一届一次、且必须贴着届末。
// 现实里的休战窗口未必长这样——角斗场的届是【周】(周一 04:00 换届),
// 而它的结算休赛要求【每天】0 点都来一次,用旧签名压根写不出来,
// 只能让业务绕开框架自己判,于是 Writable() 这类框架接口再也反映不了真实状态。
//
// 改成回调后,「什么时候算休战」完全交给榜自己;框架只问一句"现在休不休战"。
// 届末 X 秒那种老用法照样能写:
//
//	func (h *xxx) Truce(cycle int64) bool {
//	    _, e := h.Expire(cycle)
//	    return time.Now().Unix() >= e-300
//	}
//
// # 休战期内的写入一律显式报错
//
// 🔴 ZAdd 在休战期**返回 ErrTruce,不再静默返回 nil**。旧行为是个长期的坑:
// 玩家扣了票、发了奖、分数却没写进去,而调用方拿到 nil 以为成功,日志里什么都看不到。
// ZSwap / ZAdds 从一开始就没有沿用那个静默语义,现在 ZAdd 与它们对齐。
type HandleTruce interface {
	Truce(cycle int64) bool
}

// Member 批量写分的一条记录,见 Bucket.ZAdds
//
// 用值类型而不是复用 *Player:Player 带 Rank(写入时无意义),且指针切片在建榜这种
// 上千条的场景下要多做上千次分配。
type Member struct {
	Uid   string
	Score int64
}
