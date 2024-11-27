package model

import (
	"errors"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
	"github.com/hwcer/uuid"
	"server/config"
	"server/define"
	"server/share"
)

//const UpdaterRoleName = "role"

const (
	RoleModelPlug = "_model_role_plug"
)

var ITypeRole = &roleIType{IType: *NewIType(define.ITypeRole)}

func init() {
	Register(&Role{})
	it := []updater.IType{ITypeRole, ITypeItemGroup, ITypeItemPacks}
	//ROLE
	if err := updater.Register(updater.ParserTypeDocument, updater.RAMTypeAlways, &Role{}, it...); err != nil {
		logger.Panic(err)
	}
}

func NewRole() *Role {
	r := &Role{}
	r.Lv = 1
	for _, h := range Handle {
		if i, ok := h.(roleInit); ok {
			i.Init(r)
		}
	}
	return r
}

type Role struct {
	Uid     uint64          `json:"uid" bson:"_id"`
	Lv      int32           `json:"lv" bson:"lv"`              //等级
	Exp     int64           `json:"exp" bson:"exp"`            //经验
	Guid    string          `bson:"guid" json:"guid" index:""` //账号ID
	Name    string          `json:"name" bson:"name"`          //名称
	Icon    string          `json:"icon" bson:"icon"`          //头像
	Goods   map[int32]int64 `json:"goods" bson:"goods"`        //常规计数类型道具，可选使用
	Daily   RoleDaily       `json:"daily" bson:"daily"`        //通用日常数据，每日自动清理
	Record  map[int32]int64 `json:"record" bson:"record"`      //成就,并不直接返回给客户端
	Create  int64           `json:"create" bson:"create" `     //创建时间
	Update  int64           `json:"-" bson:"update" `          //最后更新时间
	Machine string          `json:"-" bson:"machine"`          //客户端机器码,用于判断是否更换机器
}

func (r *Role) Get(k string) (v any, ok bool) {
	switch k {
	case "Uid", "uid", "_id":
		return r.Uid, true
	case "Lv", "lv":
		return r.Lv, true
	case "Exp", "exp":
		return r.Exp, true
	case "Guid", "guid":
		return r.Guid, true
	case "Name", "name":
		return r.Name, true
	case "Icon", "icon":
		return r.Icon, true
	case "Goods", "goods":
		return r.Goods, true
	case "Daily", "daily":
		return r.Daily, true
	case "Record", "record":
		return r.Record, true
	case "Create", "create":
		return r.Create, true
	case "Update", "update":
		return r.Update, true
	default:
		return r.getFromHandle(k)
	}
}

func (r *Role) Set(k string, v any) (any, bool) {
	switch k {
	case "Lv", "lv":
		r.Lv = v.(int32)
	case "Exp", "exp":
		r.Exp = v.(int64)
	case "Guid", "guid":
		r.Guid = v.(string)
	case "Name", "name":
		r.Name = v.(string)
	case "Icon", "icon":
		r.Icon = v.(string)
	case "Goods", "goods":
		r.Goods = v.(map[int32]int64)
	case "Daily", "daily":
		r.Daily = v.(RoleDaily)
	case "Record", "record":
		r.Record = v.(map[int32]int64)
	case "Create", "create":
		r.Create = v.(int64)
	case "Update", "update":
		r.Update = v.(int64)
	default:
		return r.setFromHandle(k, v)
	}
	return v, true
}

func (r *Role) Loading() updater.RAMType {
	return updater.RAMTypeAlways
}

func (r *Role) TableName() string {
	return "role"
}
func (r *Role) TableOrder() int32 {
	return 100
}

func (r *Role) New(u *updater.Updater) any {
	return NewRole()
}
func (r *Role) IType(iid int32) int32 {
	return define.ITypeRole
}

func (r *Role) Field(u *updater.Updater, iid int32) (string, error) {
	if c, ok := config.Data.Role[iid]; ok {
		return c.Key, nil
	}
	return "", errors.New("config not exist")
}

func (r *Role) Getter(u *updater.Updater, data *dataset.Document, keys []string) error {
	tx := DB
	if len(keys) > 0 {
		tx = tx.Select(keys...)
	}
	uid := GetUid(u)
	if uid == 0 {
		return errors.New("Role.Getter uid not found")
	}
	v := NewRole()
	if tx = tx.Find(v, uid); tx.Error != nil {
		return tx.Error
	} else if tx.RowsAffected == 0 {
		return define.ErrRoleNotExist
	}
	data.Reset(v)
	return nil
}
func (r *Role) Setter(u *updater.Updater, data dataset.Update) error {
	uid := GetUid(u)
	if uid == 0 {
		return errors.New("Role.Setter uid not found")
	}
	tx := DB.Model(r).Update(map[string]any(data), uid)
	return tx.Error
}

type roleIType struct {
	IType
	Builder *uuid.Builder
}

func (this *roleIType) init() (err error) {
	sid := share.Options.Game.Sid
	role := &Role{}
	if tx := DB.Select("_id").Order("_id", -1).Limit(1).Find(role); tx.Error != nil {
		return tx.Error
	} else if tx.RowsAffected == 0 {
		this.Builder = uuid.New(uint16(sid), 1000)
	} else {
		this.Builder, err = uuid.Create(role.Uid, 10)
	}
	return
}

// Listener 监听升级状态
func (this *roleIType) Listener(u *updater.Updater, op *operator.Operator) {
	if op.Type == operator.TypesAdd && (op.IID == define.ItemTypeEXP || op.Key == "exp" || op.Key == "Exp") {
		lv := u.Val(define.ItemTypeLV)
		if _, ok := config.Data.Level[int32(lv+1)]; !ok {
			op.Type = operator.TypesDrop //最大等级不给经验
		} else {
			_ = u.Events.LoadOrCreate(RoleModelPlug, this.NewMiddleware)
		}
	}
}

func (this *roleIType) NewMiddleware() updater.Middleware {
	return &RoleMiddleware{}
}

type RoleMiddleware struct {
}

func (this RoleMiddleware) Emit(u *updater.Updater, t updater.EventType) bool {
	if t == updater.OnPreSubmit {
		return this.upgrade(u)
	}
	return true
}

func (this RoleMiddleware) upgrade(u *updater.Updater) bool {
	lv := int32(u.Val(define.ItemTypeLV))
	exp := u.Val(define.ItemTypeEXP)

	var newLv int32

	for i := lv + 1; ; i++ {
		if c := config.Data.Level[i]; c != nil && exp >= c.Exp {
			newLv = i
		} else {
			break
		}
	}

	if newLv > 0 {
		u.Set(define.ItemTypeLV, newLv)
	}

	return false
}
