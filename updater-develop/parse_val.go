package updater

import (
	"fmt"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
)

var hashParseHandle = make(map[operator.Types]func(*Values, *operator.Operator) error)

func init() {
	hashParseHandle[operator.TypesAdd] = hashParseAdd
	hashParseHandle[operator.TypesSub] = hashParseSub
	hashParseHandle[operator.TypesSet] = hashParseSet
	hashParseHandle[operator.TypesDel] = hashParseDel
	hashParseHandle[operator.TypesMax] = hashParseMax
	hashParseHandle[operator.TypesMin] = hashParseMin
	hashParseHandle[operator.TypesDrop] = hashParseResolve
	hashParseHandle[operator.TypesResolve] = hashParseResolve
}

func (this *Values) Parse(op *operator.Operator) (err error) {
	if err = overflow(this.Updater, this, op); err != nil {
		return
	}
	if f, ok := hashParseHandle[op.Type]; ok {
		return f(this, op)
	}
	return fmt.Errorf("hash operator type not exist:%v", op.Type.ToString())
}
func hashParseResolve(this *Values, op *operator.Operator) (err error) {
	return
}

func hashParseAdd(this *Values, op *operator.Operator) (err error) {
	r := this.Val(op.IID)
	r += op.Value
	op.Result = r
	this.dataset.Set(op.IID, r)
	return
}

func hashParseSub(this *Values, op *operator.Operator) error {
	d := this.Val(op.IID)
	r, err := this.Updater.deduct(op.IID, d, op.Value)
	if err != nil {
		return err
	}
	op.Result = r
	this.dataset.Set(op.IID, r)
	return nil
}

func hashParseSet(this *Values, op *operator.Operator) (err error) {
	op.Type = operator.TypesSet
	r := dataset.ParseInt64(op.Result)
	op.Result = r
	this.dataset.Set(op.IID, r)
	return
}

func hashParseDel(this *Values, op *operator.Operator) (err error) {
	op.Result = 0
	this.dataset.Set(op.IID, 0)
	return
}

func hashParseMax(this *Values, op *operator.Operator) (err error) {
	v := this.Val(op.IID)
	if op.Value > v {
		op.Result = op.Value
		err = hashParseSet(this, op)
	} else {
		op.Result = operator.TypesDrop
	}
	return
}

func hashParseMin(this *Values, op *operator.Operator) (err error) {
	v := this.Val(op.IID)
	if op.Value > v {
		op.Result = op.Value
		err = hashParseSet(this, op)
	} else {
		op.Type = operator.TypesDrop
	}
	return
}
