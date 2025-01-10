package updater

import (
	"fmt"
	"github.com/hwcer/updater/operator"
)

var documentParseHandle = make(map[operator.Types]func(*Document, *operator.Operator) error)

func init() {
	documentParseHandle[operator.TypesAdd] = documentParseAdd
	documentParseHandle[operator.TypesSet] = documentParseSet
	documentParseHandle[operator.TypesSub] = documentParseSub
	documentParseHandle[operator.TypesMax] = documentParseMax
	documentParseHandle[operator.TypesMin] = documentParseMin
	documentParseHandle[operator.TypesDrop] = documentParseResolve
	documentParseHandle[operator.TypesResolve] = documentParseResolve
}

func (this *Document) Parse(op *operator.Operator) (err error) {
	if err = overflow(this.Updater, this, op); err != nil {
		return
	}
	if f, ok := documentParseHandle[op.Type]; ok {
		return f(this, op)
	}
	return fmt.Errorf("document operator type not exist:%v", op.Type.ToString())
}
func documentParseResolve(this *Document, op *operator.Operator) (err error) {
	return
}
func documentParseAdd(this *Document, op *operator.Operator) (err error) {
	r, _ := this.val(op.Key)
	r += op.Value
	op.Result = r
	this.dataset.Set(op.Key, r)
	return
}

func documentParseSub(this *Document, op *operator.Operator) error {
	d, _ := this.val(op.Key)
	r, err := this.Updater.deduct(op.IID, d, op.Value)
	if err != nil {
		return err
	}
	op.Result = r
	this.dataset.Set(op.Key, r)
	return nil
}

func documentParseSet(this *Document, op *operator.Operator) (err error) {
	op.Type = operator.TypesSet
	this.dataset.Set(op.Key, op.Result)
	return
}

func documentParseMax(this *Document, op *operator.Operator) (err error) {
	v, _ := this.val(op.Key)
	if op.Value > v {
		op.Result = op.Value
		err = documentParseSet(this, op)
	} else {
		op.Type = operator.TypesDrop
	}
	return
}

func documentParseMin(this *Document, op *operator.Operator) (err error) {
	v, _ := this.val(op.Key)
	if op.Value < v {
		op.Result = op.Value
		err = documentParseSet(this, op)
	} else {
		op.Type = operator.TypesDrop
	}
	return
}
