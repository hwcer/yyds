package model

import (
	"errors"
	"fmt"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo/update"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/yyds/game/share"
)

const dailyValuesFormat = "val.%v"

func init() {
	Register(&Daily{})
}

type Daily struct {
	Model  `bson:"inline"`
	Value  map[int32]int64 `bson:"val" json:"val"`  //日常记录
	Signup int32           `bson:"signup" json:"-"` //玩家注册日期，可以配合IID查留存
}

func (this *Daily) New(u *updater.Updater, iid int32) (r *Daily, err error) {
	r = &Daily{}
	r.Model.Init(u, iid)
	r.OID, err = Unique(u, iid)
	r.Value = map[int32]int64{}
	return
}

func (this *Daily) IType(int32) int32 {
	return share.ITypeDaily
}

func (this *Daily) LoadOrCreate(u *updater.Updater, iid int32) (r *Daily, err error) {
	if iid == 0 {
		iid, _ = times.Sign(0)
	}
	if r, err = this.New(u, iid); err != nil {
		return
	}
	if tx := DB.Find(r, r.OID); tx.Error != nil {
		return nil, tx.Error
	} else if tx.RowsAffected == 0 {
		doc := u.Handle(share.ITypeRole).(*updater.Document)
		if doc == nil {
			return nil, errors.New("daily Getter Handle(Role) empty")
		}
		create := doc.Val("create")
		r.Signup, _ = times.Timestamp(create).Sign(0)
		if tx = DB.Create(r); tx.Error != nil {
			return nil, tx.Error
		}
	}
	return
}

func (this *Daily) Getter(u *updater.Updater, values *dataset.Values, keys []int32) error {
	//内存模式只会拉所有
	//if len(keys) > 0 {
	//	return errors.New("daily Getter 参数keys应该为空")
	//}
	t := times.New(u.Time)
	expire, err := t.Expire(times.ExpireTypeDaily, 1)
	if err != nil {
		return err
	}
	iid, _ := t.Sign(0)
	var daily *Daily
	if daily, err = this.LoadOrCreate(u, iid); err != nil {
		return err
	}
	values.Reset(daily.Value, expire.Unix())
	return nil
}

func (this *Daily) Setter(u *updater.Updater, data dataset.Data, expire int64) error {
	if len(data) == 0 {
		return nil
	}
	t := times.Timestamp(expire)
	iid, _ := t.Sign(0)
	if iid <= 0 {
		return fmt.Errorf("daily expire error:%v", expire)
	}
	up := update.Update{}
	for k, v := range data {
		up.Set(fmt.Sprintf(dailyValuesFormat, k), v)
	}
	up.Set("update", u.Time.Unix())
	oid, err := Unique(u, iid)
	if err != nil {
		return err
	}
	tx := DB.Model(this).Update(up, oid)
	return tx.Error
}

// Count 统计一段时间内的日常总和
func (this *Daily) Count(uid uuid.UUID, keys []int32, start, end *times.Times) (r map[int32]int64, err error) {
	if len(keys) == 0 {
		return nil, values.Errorf(0, "keys empty")
	}
	var rows []*Daily
	tx := DB.Model(this).Where("uid = ?", uid)
	if start != nil {
		s, _ := start.Sign(0)
		tx = tx.Where("iid >= ?", s)
	}
	if end != nil {
		s, _ := end.Sign(0)
		tx = tx.Where("iid <= ?", s)
	}
	var fields []string
	for _, k := range keys {
		fields = append(fields, fmt.Sprintf(dailyValuesFormat, k))
	}
	tx = tx.Select(fields...)
	if tx = tx.Find(&rows); tx.Error != nil {
		return nil, tx.Error
	}
	r = map[int32]int64{}
	for _, row := range rows {
		for k, v := range row.Value {
			r[k] += v
		}
	}
	return
}

// Weekly 周统计  offset=0当前周, -1:上周
func (this *Daily) Weekly(uid uuid.UUID, keys []int32, offset int) (r map[int32]int64, err error) {
	star := times.Weekly(offset)
	end, _ := star.Expire(times.ExpireTypeWeekly, 1)
	return this.Count(uid, keys, star, end)
}

// Monthly 月统计统计 同周,但是每月天数不一样,按自然月算
func (this *Daily) Monthly(uid uuid.UUID, keys []int32, offset int) (r map[int32]int64, err error) {
	star := times.Monthly(offset)
	end, _ := star.Expire(times.ExpireTypeMonthly, 1)
	return this.Count(uid, keys, star, end)
}
