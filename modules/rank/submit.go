package rank

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hwcer/logger"
)

// ErrSubmitted 该期正在结算或已在本进程内结算完成
var ErrSubmitted = errors.New("排行榜该期正在结算或已结算")

// checkUnsettledRanks 检查是否有未结算的排行榜
func (this *Bucket) checkUnsettledRanks() {
	key := this.RedisSettlementKey()
	if exists, err := Redis.Exists(context.Background(), key).Result(); err != nil {
		logger.Error("检查结算记录失败: %v", err)
		return
	} else if exists == 0 {
		return
	}
	// 遍历整个HASH表，检查所有周期的结算状态
	fields, err := Redis.HGetAll(context.Background(), key).Result()
	if err != nil {
		logger.Error("获取结算记录失败: %v", err)
		return
	}
	var unsettledCycles []int64
	var cycle int64
	var status int
	for cycleStr, statusStr := range fields {
		cycle, err = strconv.ParseInt(cycleStr, 10, 64)
		if err != nil {
			logger.Error("解析周期值失败: %v", err)
			continue
		}
		status, err = strconv.Atoi(statusStr)
		if err != nil {
			logger.Error("解析结算状态失败: %v", err)
			continue
		}
		if status == 0 {
			logger.Info("发现未结算的排行榜: %v, 周期: %v", this.zName, cycle)
			unsettledCycles = append(unsettledCycles, cycle)
		}
	}
	if len(unsettledCycles) == 0 {
		return
	}
	this.zMutex.Lock()
	for _, cycle = range unsettledCycles {
		this.submits = append(this.submits, NewStatement(cycle, 0, 0))
	}
	this.zMutex.Unlock()

}

func (this *Bucket) RedisSettlementKey() string {
	return fmt.Sprintf("%v-rs-%v-%v", Options.ShareId, Options.ServerId, this.zKey)
}

// Submit 手动触发指定届的结算,用于自动结算失败后的人工补偿
//
// 只是把该届加入待结算队列,真正的结算由心跳协程串行执行,因此本方法立即返回,
// 拿不到结算结果;结算失败会记录 Alert 日志,并保留Redis中的未结算记录等待下次重试
func (this *Bucket) Submit(cycle int64) error {
	if cycle == 0 {
		return fmt.Errorf("排行榜[%v]手动结算必须指定期数", this.zName)
	}
	this.zMutex.Lock()
	defer this.zMutex.Unlock()
	for _, v := range this.submits {
		if v.zCycle == cycle {
			return nil //已在队列中,不重复入队
		}
	}
	this.submits = append(this.submits, NewStatement(cycle, 0, 0))
	return nil
}

// maySubmit 执行结算并收尾:清除结算记录 + 设置数据过期时间
//
// 只在心跳协程中调用。结算是全局唯一执行者,不需要额外的并发保护:
// 手动补发也只是入队,不会自己执行
func (this *Bucket) maySubmit(stmt *Statement) {
	if stmt.submit.Load() {
		return
	}
	cycle := stmt.zCycle
	expire, err := this.handle.Submit(this, cycle)
	if err != nil {
		// 结算失败不要修改状态，避免重复结算的情况应该在业务层面实现，比如唯一邮件ID
		// 结算记录保留在Redis中,重启后会自动重试,也可由业务层调用 Bucket.Submit 手动补发
		logger.Alert("结算失败: %v", err)
		return
	}
	stmt.submit.Store(true)
	// 结算完成后删除Redis hash表中的记录
	key := this.RedisSettlementKey()
	if err = Redis.HDel(context.Background(), key, strconv.FormatInt(cycle, 10)).Err(); err != nil {
		logger.Alert("删除结算记录失败: %v", err)
	}
	// 按 Submit 返回的删除时间设置过期,过期时间无效时使用默认保留时长兜底
	rk := this.RedisRankKey(cycle)
	if expire > time.Now().Unix() {
		err = Redis.ExpireAt(context.Background(), rk, time.Unix(expire, 0)).Err()
	} else {
		err = Redis.Expire(context.Background(), rk, DefaultRetention).Err()
	}
	if err != nil {
		logger.Alert("设置排行榜数据过期时间失败: %v", err)
	}
}

func (this *Bucket) changeCycle(cycle int64) {
	// handle.Expire 是业务实现的回调,不能在持锁时调用:
	// 若其内部再访问本 Bucket(如 Cycle/ZAdd)会自死锁,并拖死整个心跳协程
	s, e := this.handle.Expire(cycle)

	this.zMutex.Lock()
	stmt := this.zStmt.Load()
	// 只允许期数递增。并发下两个协程可能读到不同的期数,晚到的旧值若覆盖回去,
	// 会把正在进行的一届提前结算,并让后续 ZAdd 写进已废弃的KEY
	if stmt != nil && cycle <= stmt.zCycle {
		this.zMutex.Unlock()
		return
	}
	this.zStmt.Store(NewStatement(cycle, s, e))
	if stmt == nil || stmt.submit.Load() {
		this.zMutex.Unlock()
		return
	}
	this.zMutex.Unlock()

	// 必须先落结算标记,再入队。若顺序反过来,心跳协程可能在 HSet 之前就完成结算并 HDel,
	// 之后 HSet 才落库,留下一条永不清除的未结算记录,重启后重复结算
	// 存储对应周期的结算状态，0表示未结算
	key := this.RedisSettlementKey()
	if err := Redis.HSet(context.Background(), key, stmt.zCycle, 0).Err(); err != nil {
		logger.Error("保存结算记录失败: %v", err)
	}
	// 切换届时将上一届加入待结算队列
	this.zMutex.Lock()
	this.submits = append(this.submits, stmt)
	this.zMutex.Unlock()
}
