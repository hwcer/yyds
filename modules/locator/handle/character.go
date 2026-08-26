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

// charactersMax 一次批量查询的 uid 上限。
//
// 调用方是社交玩法的列表界面（公会成员、好友、申请），它们本身都有各自的容量上限，
// 200 足够覆盖。设这个数不是怕慢——主键点查很便宜——而是防止调用方把一个
// 无界的集合整个丢进来（比如「全服玩家」），那种请求应该在调用方就被拦住。
const charactersMax = 200

// CharactersArgs Gets 的入参
type CharactersArgs struct {
	Uids []string `json:"uids"`
}

// Gets 按 uid 批量取角色目录。
//
// 与 Find（按 guid 取一个账号下的角色）不是一回事：这是**跨账号、跨服**的批量点查。
//
// # 为什么社交玩法需要它
//
// 公会成员列表、好友列表、入会申请列表都要显示一批人的昵称 / 等级 / 头像，
// 而这些人**可能分布在不同区服**（好友本就是跨服的）。游戏服只认得本区玩家，
// 问它拿不到别区的人；本模块是全服角色目录，这正是它存在的理由。
//
// # 只回展示字段
//
// guid（账号 ID）与 create 不下发：调用方要的是「这个角色长什么样」，
// 不是「他属于哪个账号」。少给一个字段，少一处泄漏面。
func (this *character) Gets(c *cosrpc.Context) interface{} {
	args := &CharactersArgs{}
	if err := c.Bind(args); err != nil {
		return err
	}
	if len(args.Uids) == 0 {
		return []*model.Character{}
	}
	if len(args.Uids) > charactersMax {
		return c.Error(fmt.Sprintf("too many uids: %d > %d", len(args.Uids), charactersMax))
	}
	var rows []*model.Character
	tx := db.Select("_id", "sid", "online", "update", "attach")
	if tx = tx.Find(&rows, args.Uids); tx.Error != nil {
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
