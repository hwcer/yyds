package updater

import (
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
)

// 溢出判断
func overflow(update *Updater, handle Handle, op *operator.Operator) (err error) {
	if op.Type != operator.TypesAdd || op.IID == 0 {
		return nil
	}
	it := handle.IType(op.IID)
	if it == nil {
		return ErrITypeNotExist(op.IID)
	}
	val := dataset.ParseInt64(op.Value)
	num := handle.Val(op.IID)
	tot := val + num
	imax := Config.IMax(op.IID)
	if imax > 0 && tot > imax {
		n := tot - imax
		if n > val {
			n = val //imax有改动
		}
		val -= n
		op.Value = val
		if resolve, ok := it.(ITypeResolve); ok {
			if err = resolve.Resolve(update, op.IID, n); err != nil {
				return
			} else {
				n = 0
			}
		}
		if n > 0 {
			//this.Adapter.overflow[cache.IID] += overflow
		}
	}
	if val == 0 {
		op.Type = operator.TypesResolve
	}
	return
}
