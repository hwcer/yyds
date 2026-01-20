package rank

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hwcer/logger"
)

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
	return fmt.Sprintf("%v-rs-%v-%v", Options.ShareId, Options.ServerId, this.zName)
}

func (this *Bucket) maySubmit(stmt *Statement) {
	if stmt.submit {
		return
	}
	var err error
	if err = this.handle.Submit(this, stmt.zCycle); err != nil {
		logger.Alert("结算失败: %v", err)
		// 结算失败不要修改状态，避免重复结算的情况应该在业务层面实现，比如唯一邮件ID
		return
	}
	// 结算完成后删除Redis hash表中的记录
	key := this.RedisSettlementKey()
	stmt.submit = true
	if err = Redis.HDel(context.Background(), key, strconv.FormatInt(stmt.zCycle, 10)).Err(); err != nil {
		logger.Alert("删除结算记录失败: %v", err)
	}
	// 结算完成时设置排行榜数据过期时间为7天
	rk := this.RedisRankKey(stmt.zCycle)
	if err = Redis.Expire(context.Background(), rk, 7*24*time.Hour).Err(); err != nil {
		logger.Alert("设置排行榜数据过期时间失败: %v", err)
	}
}

func (this *Bucket) changeCycle(cycle int64) {
	this.zMutex.Lock()
	defer this.zMutex.Unlock()
	stmt := this.Statement
	if stmt != nil && cycle == stmt.zCycle {
		return
	}
	s, e := this.handle.Expire(cycle)
	this.Statement = NewStatement(cycle, s, e)
	if stmt == nil || stmt.submit {
		return
	}
	// 切换界时将上一届的信息写入到Redis hash表
	this.submits = append(this.submits, stmt)
	key := this.RedisSettlementKey()
	// 存储对应周期的结算状态，0表示未结算
	status := 0
	if err := Redis.HSet(context.Background(), key, stmt.zCycle, status).Err(); err != nil {
		logger.Error("保存结算记录失败: %v", err)
	}
}
