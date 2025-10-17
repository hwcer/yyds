package master

import (
	"fmt"

	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/cosweb"
	"github.com/hwcer/yyds/locator/model"
)

func init() {
	_ = Service.Register(&Character{})
}

type Character struct {
}

func (this *Character) Caller(node *registry.Node, c *cosweb.Context) interface{} {
	method := node.Method()
	f := method.(func(*Character, *cosweb.Context) interface{})
	return f(this, c)
}

type CharacterPageArgs struct {
	*cosmo.Paging
	Sid   int32  `json:"sid"`
	Key   string `json:"key"`   //查询字段名
	Value string `json:"value"` //查询值
}

func (this *Character) Page(c *cosweb.Context) interface{} {
	args := &CharacterPageArgs{}
	if err := c.Bind(args); err != nil {
		return err
	}
	args.Paging.Rows = make([]*model.Character, 0)
	tx := db.Order("online", -1)
	if args.Sid != 0 {
		tx = tx.Where("sid = ?", args.Sid)
	}
	args.Paging.Init(100)
	if args.Key != "" && args.Value != "" {
		tx = tx.Where(fmt.Sprintf("%v = ?", args.Key), args.Value)
	}
	if tx = tx.Page(args.Paging); tx.Error != nil {
		return values.Error(tx.Error)
	}
	return args.Paging
}

// Find 根据guid查询角色列表
func (this *Character) Find(c *cosweb.Context) interface{} {
	guid := c.GetString("guid")
	if guid == "" {
		return values.Errorf(0, "guid is required")
	}
	reply := make([]*model.Character, 0)
	tx := db.Order("update", -1)
	tx = tx.Where("guid = ?", guid).Find(&reply)
	if tx.Error != nil {
		return values.Error(tx.Error)
	}
	return reply
}
