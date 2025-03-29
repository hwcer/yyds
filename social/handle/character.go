package handle

import (
	"fmt"
	"github.com/hwcer/cosgo/registry"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosmo/update"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/social/model"
	"time"
)

func init() {
	Register(&character{})
}

type character struct {
}
type CharacterArgs struct {
	model.Character
	First int32 `json:"first"`
}

func (this *character) Caller(node *registry.Node, handle *xshare.Context) interface{} {
	f := node.Method().(func(*character, *xshare.Context) interface{})
	return f(this, handle)
}

func (this *character) Count(c *xshare.Context) interface{} {
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

func (this *character) Find(c *xshare.Context) interface{} {
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

// Invite 我邀请的人
func (this *character) Invite(c *xshare.Context) interface{} {
	uid := c.GetInt64("uid")
	if uid == 0 {
		return c.Error("uid required")
	}
	size := c.GetInt32("size")
	var rows []*model.Character
	tx := db.Order("update", -1)
	if size > 0 {
		tx = tx.Limit(int(size))
	}
	if tx = tx.Find(&rows, "invite = ?", uid); tx.Error != nil {
		return c.Error(tx.Error)
	}
	return rows
}

func (this *character) Create(c *xshare.Context) interface{} {
	v := &CharacterArgs{}
	if err := c.Bind(v); err != nil {
		return err
	}
	if v.Uid == "" || v.Guid == "" {
		return c.Error("uid or guid required")
	}
	if v.Create == 0 {
		v.Create = time.Now().Unix()
	}
	//邀请人信息
	if v.Invite != "" {
		var n int64
		if tx := db.Model(&v.Character).Count(&n, v.Invite); tx.Error != nil {
			return tx.Error
		} else if n == 0 {
			return c.Error("invite not exist")
		}
	}

	if tx := db.Create(&v.Character); tx.Error != nil {
		return c.Error(tx.Error)
	}
	ts := times.Timestamp(v.Create)
	sign, _ := ts.Sign(0)
	Analyse := model.NewAnalyse(v.Sid, sign)
	up := update.Update{}
	up.Inc("create", 1)
	up.SetOnInert("sid", v.Sid)
	up.SetOnInert("day", sign)
	if tx := db.Model(Analyse).Upsert().Update(up, Analyse.Id); tx.Error != nil {
		return c.Error(tx.Error)
	}
	return true
}

func (this *character) Update(c *xshare.Context) interface{} {
	v := &CharacterArgs{}
	if err := c.Bind(v); err != nil {
		return err
	}
	if v.Uid == "" {
		return c.Error("uid or guid required")
	}

	if tx := db.Omit("_id", "sid", "guid", "create", "invite").Update(&v.Character, v.Uid); tx.Error != nil {
		return c.Error(tx.Error)
	}
	if v.First < 1 {
		return nil
	}

	ts := times.Timestamp(v.Create)
	sign, _ := ts.Sign(0)
	Analyse := model.NewAnalyse(v.Sid, sign)
	create := ts.Daily(0)
	s := v.Update - create.Unix()
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
	up.SetOnInert("sid", v.Sid)
	up.SetOnInert("day", sign)
	if tx := db.Model(Analyse).Upsert().Update(up, Analyse.Id); tx.Error != nil {
		return c.Error(tx.Error)
	}
	return true
}
