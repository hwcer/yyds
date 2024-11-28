package model

import (
	"errors"
	"github.com/hwcer/cosmo/update"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
	"github.com/hwcer/yyds/kernel/config"
	"time"
)

type TaskStatus int8

const (
	TaskStatusNone  TaskStatus = 0 //无
	TaskStatusStart TaskStatus = 1 //即时任务进行中
	//TaskStatusFinish TaskStatus = 2 //已经完成
)

func init() {
	Register(&Task{})
}

type Task struct {
	Model  `bson:"inline"`
	Value  int32      `bson:"val" json:"val"`        //完成次数，一般 0/1 除非可以多次完成的任务，通过重置 Status = 1
	Target int64      `bson:"tar" json:"tar"`        //任务进度,仅仅即时任务有效
	Status TaskStatus `bson:"status" json:"status" ` //0-无，1-进行中
}

func (this *Task) Get(k string) (any, bool) {
	switch k {
	case "Status", "status":
		return this.Status, true
	case "Value", "val":
		return this.Value, true
	case "Target", "tar":
		return this.Target, true
	default:
		return this.Model.Get(k)
	}
}

// Set 更新器
func (this *Task) Set(k string, v any) (any, bool) {
	switch k {
	case "Status", "status":
		this.Status = v.(TaskStatus)
	case "Value", "val":
		this.Value = dataset.ParseInt32(v)
	case "Target", "tar":
		this.Target = dataset.ParseInt64(v)
	default:
		return this.Model.Set(k, v)
	}
	return v, true
}

func (this *Task) IType(int32) int32 {
	return config.ITypeTask
}

// ----------------- 作为MODEL方法--------------------

// Clone 复制对象,可以将NEW新对象与SET操作解绑
func (this *Task) Clone() any {
	r := *this
	return &r
}

func (this *Task) Upsert(u *updater.Updater, op *operator.Operator) bool {
	return true
}

func (this *Task) Getter(u *updater.Updater, coll *dataset.Collection, keys []string) error {
	uid, _ := u.Uid().(uint64)
	if uid == 0 {
		return errors.New("Task.Getter uid not found")
	}
	tx := DB.Where("uid = ?", uid)
	if len(keys) > 0 {
		tx = tx.Where("_id IN ?", keys)
	} else {
		tx = tx.Where("status = ?", TaskStatusStart)
	}
	tx = tx.Omit("uid", "update")
	var rows []*Task
	if tx = tx.Find(&rows); tx.Error != nil {
		return tx.Error
	}
	for _, v := range rows {
		coll.Receive(v.OID, v)
	}
	return nil
}
func (this *Task) Setter(u *updater.Updater, bulkWrite dataset.BulkWrite) error {
	return bulkWrite.Save()
}
func (this *Task) BulkWrite(u *updater.Updater) dataset.BulkWrite {
	return NewBulkWrite(this)
}

func (this *Task) BulkWriteFilter(up update.Update) {
	if !up.Has(update.UpdateTypeSet, "update") {
		this.Update = time.Now().Unix()
		up.Set("update", this.Update)
	}
}
