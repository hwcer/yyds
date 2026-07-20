package rank

import (
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo"
)

// Get 获取排行榜
func Get(name any) *Bucket {
	return Master.Get(name)
}

// ZAdd 设置排行榜积分,使用最终值,而不是增量
//
//	name 排行榜名称
func ZAdd(name any, cycle int64, uid string, score int64) error {
	if uid == "" {
		return values.Error("uid empty")
	}
	w := Master.Get(name)
	if w == nil {
		return values.Error("Rank not exist")
	}
	if err := w.ZAdd(cycle, uid, score); err != nil {
		return err
	}
	return nil
}

func ZCard(name any, cycle int64) (int64, error) {
	w := Master.Get(name)
	if w == nil {
		return 0, values.Error("Rank not exist")
	}
	return w.ZCard(cycle)
}

// ZPage 区间数据 按分页逻辑
func ZPage(name any, cycle int64, paging *cosmo.Paging) error {
	w := Master.Get(name)
	if w == nil {
		return values.Errorf(0, "Rank not exist")
	}
	return w.ZPage(cycle, paging)
}

// ZRank 返回个人名次
func ZRank(name any, cycle int64, uid string, withScore bool) (*Player, error) {
	w := Master.Get(name)
	if w == nil {
		return nil, values.Errorf(0, "Rank not exist")
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
	w := Master.Get(name)
	if w == nil {
		return nil, values.Errorf(0, "Rank not exist")
	}
	return w.ZRange(cycle, s, e)
}
func ZPlayer(name any, cycle int64, rank int64) (r *Player, err error) {
	w := Master.Get(name)
	if w == nil {
		return nil, values.Errorf(0, "Rank not exist")
	}
	return w.ZPlayer(cycle, rank)
}

// Cycle 当前第几届
func Cycle(name any, skip int64) int64 {
	w := Master.Get(name)
	if w == nil {
		return 0
	}
	return w.handle.Cycle(skip)
}

// Expire 每一届界的开始结束时间
func Expire(name any, cycle int64) (s, e int64) {
	w := Master.Get(name)
	if w == nil {
		return 0, 0
	}
	return w.handle.Expire(cycle)
}

// Submit 手动触发指定届结算,用于自动结算失败后的人工补偿
func Submit(name any, cycle int64) error {
	w := Master.Get(name)
	if w == nil {
		return values.Error("Rank not exist")
	}
	return w.Submit(cycle)
}

func Writable(name any, cycle int64) (r bool) {
	w := Master.Get(name)
	if w == nil {
		return false
	}
	return w.Writable(cycle)
}
