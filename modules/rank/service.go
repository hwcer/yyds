package rank

import (
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo"
)

// Get 获取排行榜
func Get(name any) *Bucket {
	return Master.Get(name)
}

// Started 排行榜模块是否已启动(Start 调用过、Redis 已就绪)。
//
// 🔴 **Get(name) != nil 不代表已启动**:Register 在各模块的 init 里就把桶填进
// Master 了,而 Start 要等 Redis 配置就绪才调用。业务若只判 Get 非空就去读写,
// 未配 Redis 的环境下会在 client.XXX 上空指针——而那时调用栈里只有框架帧,
// 极难定位(2026-08-21 踩过)。判"能不能用"一律用本函数。
func Started() bool {
	return client != nil
}

// bucket 取桶并确认模块已启动。
//
// 两者任一不满足都返回明确错误,而不是让调用方拿着一个能取到、
// 却会在第一次访问 Redis 时崩掉的桶。
func bucket(name any) (*Bucket, error) {
	if client == nil {
		return nil, ErrNotStarted
	}
	w := Master.Get(name)
	if w == nil {
		return nil, values.Errorf(0, "Rank not exist:%v", name)
	}
	return w, nil
}

// ZAdd 设置排行榜积分,使用最终值,而不是增量
//
//	name 排行榜名称
func ZAdd(name any, cycle int64, uid string, score int64) error {
	if uid == "" {
		return values.Error("uid empty")
	}
	w, err := bucket(name)
	if err != nil {
		return err
	}
	if err := w.ZAdd(cycle, uid, score); err != nil {
		return err
	}
	return nil
}

// ZAdds 批量写分,见 Bucket.ZAdds
//
// 返回实际写入条数;休战期返回 ErrTruce,届已过期返回 ErrCycleExpired
// (与单条 ZAdd 的静默语义【有意不同】,原因见 Bucket.ZAdds)。
func ZAdds(name any, cycle int64, members []Member) (int, error) {
	w, err := bucket(name)
	if err != nil {
		return 0, err
	}
	return w.ZAdds(cycle, members)
}

// ZSwap 原子对调两名成员的分数,返回各自对调【前】的原始分数
//
// 详见 Bucket.ZSwap。错误按 errors.Is 区分:
// ErrTruce / ErrSwapMemberMissing / ErrSwapCondition
func ZSwap(name any, cycle int64, a, b string, cond SwapCond) (scoreA, scoreB int64, err error) {
	w, err := bucket(name)
	if err != nil {
		return 0, 0, err
	}
	return w.ZSwap(cycle, a, b, cond)
}

func ZCard(name any, cycle int64) (int64, error) {
	w, err := bucket(name)
	if err != nil {
		return 0, err
	}
	return w.ZCard(cycle)
}

// ZPage 区间数据 按分页逻辑
func ZPage(name any, cycle int64, paging *cosmo.Paging) error {
	w, err := bucket(name)
	if err != nil {
		return err
	}
	return w.ZPage(cycle, paging)
}

// ZRank 返回个人名次
func ZRank(name any, cycle int64, uid string, withScore bool) (*Player, error) {
	w, err := bucket(name)
	if err != nil {
		return nil, err
	}
	if r, err := w.ZRank(cycle, uid, withScore); err != nil {
		return nil, err
	} else {
		// 确保查询成功时返回nil错误，避免上层调用失败
		return r, nil
	}
}

// ZRange 区间数据
func ZRange(name any, cycle int64, s, e int64) (r []*Player, err error) {
	w, err := bucket(name)
	if err != nil {
		return nil, err
	}
	return w.ZRange(cycle, s, e)
}
func ZPlayer(name any, cycle int64, rank int64) (r *Player, err error) {
	w, err := bucket(name)
	if err != nil {
		return nil, err
	}
	return w.ZPlayer(cycle, rank)
}

// Cycle 当前第几届
// Cycle 当前第几届。
//
// 只算周期、不碰 Redis,故【不要求模块已启动】——未配 Redis 的环境下届号照样算得出来,
// 业务拿它做展示/判断都不受影响。
func Cycle(name any, skip int64) int64 {
	w := Master.Get(name)
	if w == nil {
		return 0
	}
	return w.handle.Cycle(skip)
}

// Expire 每一届界的开始结束时间
// Expire 本届起止。同 Cycle,只算周期、不碰 Redis,不要求模块已启动。
func Expire(name any, cycle int64) (s, e int64) {
	w := Master.Get(name)
	if w == nil {
		return 0, 0
	}
	return w.handle.Expire(cycle)
}

// Submit 手动触发指定届结算,用于自动结算失败后的人工补偿
func Submit(name any, cycle int64) error {
	w, err := bucket(name)
	if err != nil {
		return err
	}
	return w.Submit(cycle)
}

// Writable 当前是否可写(不在休战期)。同 Cycle,不碰 Redis。
func Writable(name any, cycle int64) (r bool) {
	w := Master.Get(name)
	if w == nil {
		return false
	}
	return w.Writable(cycle)
}
