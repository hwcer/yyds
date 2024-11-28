package model

import (
	"github.com/hwcer/cosmo"
	"github.com/hwcer/updater/dataset"
)

func NewBulkWrite(model any, filter ...cosmo.BulkWriteUpdateFilter) *BulkWrite {
	bw := &BulkWrite{}
	bw.BulkWrite = *DB.BulkWrite(model, filter...)
	return bw
}

type BulkWrite struct {
	cosmo.BulkWrite
}

func (this *BulkWrite) Update(data dataset.Update, where ...any) {
	this.BulkWrite.Update(map[string]any(data), where...)
}
