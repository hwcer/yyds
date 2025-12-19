package master

import (
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/cosweb"
	"github.com/hwcer/yyds/locator/model"
)

func init() {
	_ = Service.Register(&Analyse{})
}

type Analyse struct {
}

func (this *Analyse) Caller(node *registry.Node, c *cosweb.Context) interface{} {
	method := node.Method()
	f := method.(func(*Analyse, *cosweb.Context) interface{})
	return f(this, c)
}

type AnalysePageArgs struct {
	cosmo.Paging
	STime string `json:"STime"` //开始时间
	ETime string `json:"ETime"` //结束时间
}

func (this *Analyse) Page(c *cosweb.Context) interface{} {
	sid := c.GetInt32("sid", cosweb.RequestDataTypeQuery)

	args := &AnalysePageArgs{}
	if err := c.Bind(args); err != nil {
		return err
	}
	args.Paging.Init(100)
	tx := db.Model(&model.Analyse{})
	if sid > 0 {
		tx = tx.Where("sid", sid)
	}
	if args.STime != "" {
		if st, err := times.Parse(args.STime, times.DateLayout); err != nil {
			return err
		} else {
			v, _ := st.Sign(0)
			tx = tx.Where("day>=?", v)
		}
	}
	if args.ETime != "" {
		if st, err := times.Parse(args.ETime, times.DateLayout); err != nil {
			return err
		} else {
			v, _ := st.Sign(0)
			tx = tx.Where("day<=?", v)
		}
	}
	tx = tx.Order("day", -1)
	var rows []*model.Analyse
	args.Paging.Rows = &rows

	if tx = tx.Page(&args.Paging); tx.Error != nil {
		return values.Error(tx.Error)
	}
	return args.Paging
}
