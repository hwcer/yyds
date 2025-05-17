package player

import "github.com/hwcer/updater/operator"

type Dirty struct {
	dict []*operator.Operator
}

func (d *Dirty) Push(opts ...*operator.Operator) {
	d.dict = append(d.dict, opts...)
}

func (d *Dirty) Pull() []*operator.Operator {
	r := d.dict
	d.dict = nil
	return r
}
