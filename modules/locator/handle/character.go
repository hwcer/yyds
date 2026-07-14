package handle

import (
	"fmt"
	"time"

	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo/update"
	"github.com/hwcer/cosrpc"
	"github.com/hwcer/yyds/modules/locator/model"
)

func init() {
	Register(&character{})
}

type character struct {
}

func (this *character) Caller(node *registry.Node, handle *cosrpc.Context) interface{} {
	f := node.Method().(func(*character, *cosrpc.Context) interface{})
	return f(this, handle)
}

func (this *character) Count(c *cosrpc.Context) interface{} {
	guid := c.GetString("guid")
	if guid == "" {
		return c.Error("guid required")
	}
	var n int64
	if tx := db.Model(&model.Character{}).Count(&n, "guid = ?", guid); tx.Error != nil {
		return c.Error(tx.Error)
	}
	return n
}

func (this *character) Find(c *cosrpc.Context) interface{} {
	guid := c.GetString("guid")
	if guid == "" {
		return c.Error("guid required")
	}
	var rows []*model.Character
	if tx := db.Order("update", -1).Find(&rows, "guid = ?", guid); tx.Error != nil {
		return c.Error(tx.Error)
	}
	return rows
}

func (this *character) Create(c *cosrpc.Context) interface{} {
	v := &model.Character{}
	if err := c.Bind(v); err != nil {
		return err
	}
	if v.Uid == "" || v.Guid == "" {
		return c.Error("uid or guid required")
	}
	if v.Create == 0 {
		v.Create = time.Now().Unix()
	}
	if v.Online == 0 {
		v.Online = v.Create
	}
	if v.Update == 0 {
		v.Update = v.Create
	}
	if len(v.Attach) == 0 {
		v.Attach = values.Values{}
	}
	if tx := db.Create(v); tx.Error != nil {
		return c.Error(tx.Error)
	}

	ts := times.Unix(v.Create)
	sign, _ := ts.Sign(0)
	Analyse := model.NewAnalyse(v.Sid, sign)
	up := update.Update{}
	up.Inc("create", 1)
	up.SetOnInsert("sid", v.Sid)
	up.SetOnInsert("day", sign)
	if tx := db.Model(Analyse).Upsert().Update(up, Analyse.Id); tx.Error != nil {
		return c.Error(tx.Error)
	}
	return true
}

// Online 角色上线
func (this *character) Online(c *cosrpc.Context) interface{} {

	args := &model.Character{}
	if err := c.Bind(args); err != nil {
		return err
	}

	v := &model.Character{}
	if tx := db.Select("create", "online", "sid").Find(v, args.Uid); tx.Error != nil {
		return tx.Error
	} else if tx.RowsAffected == 0 {
		return c.Error("character not found")
	}

	now := time.Now().Unix()
	u := args.GetUpdate()
	u["online"] = now

	if tx := db.Model(v).Where(args.Uid).Update(u); tx.Error != nil {
		return tx.Error
	}

	today := times.Daily(0).Now().Unix()
	if v.Online > today {
		return nil
	}

	ts := times.Unix(v.Create)
	sign, _ := ts.Sign(0)
	Analyse := model.NewAnalyse(v.Sid, sign)
	create := ts.Daily(0)
	s := now - create.Now().Unix()
	if s <= 0 {
		return nil
	}
	dau := int32(s/86400 + 1)
	if _, ok := model.DAU[dau]; !ok {
		return nil
	}
	key := fmt.Sprintf("active.%v", dau)
	up := update.Update{}
	up.Inc(key, 1)
	up.SetOnInsert("sid", v.Sid)
	up.SetOnInsert("day", sign)
	if tx := db.Model(Analyse).Upsert().Update(up, Analyse.Id); tx.Error != nil {
		return c.Error(tx.Error)
	}
	return true
}

// Update 更新角色信息
func (this *character) Update(c *cosrpc.Context) interface{} {
	v := &model.Character{}
	if err := c.Bind(v); err != nil {
		return err
	}
	if v.Uid == "" {
		return c.Error("uid or guid required")
	}
	if tx := db.Model(v).Update(v.GetUpdate(), v.Uid); tx.Error != nil {
		return c.Error(tx.Error)
	}
	return true
}
