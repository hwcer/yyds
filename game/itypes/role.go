package itypes

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/operator"
	"github.com/hwcer/yyds/game/config"
	"github.com/hwcer/yyds/game/model"
)

const (
	RoleModelPlug = "_model_role_plug"
)

var Role = &roleIType{IType: NewIType(config.ITypeRole)}

func init() {
	it := []updater.IType{Role, ItemsGroup, ItemsPacks}
	//ROLE
	if err := updater.Register(updater.ParserTypeDocument, updater.RAMTypeAlways, &model.Role{}, it...); err != nil {
		logger.Panic(err)
	}
}

type roleIType struct {
	*IType
	Upgrade roleUpgrade
	Builder *uuid.Builder
}
type roleUpgrade interface {
	Verify(u *updater.Updater, exp int64) (newExp int64)    //获得经验时进行检查
	Submit(u *updater.Updater, lv, exp int64) (newLv int64) //判断升级，返回新的等级
}

func (this *roleIType) init() (err error) {
	sid := options.Game.Sid
	role := &model.Role{}
	if tx := model.DB.Select("_id").Order("_id", -1).Limit(1).Find(role); tx.Error != nil {
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
	if op.Type == operator.TypesAdd && (op.Key == "exp" || op.Key == "Exp") {
		if this.Upgrade == nil {
			logger.Alert("ITypes.Options.RoleUpgrade is nil")
			return
		}
		if exp := this.Upgrade.Verify(u, op.Value); exp > 0 {
			op.Value = exp
			_ = u.Events.LoadOrCreate(RoleModelPlug, this.NewMiddleware)
		} else {
			op.Type = operator.TypesDrop //最大等级不给经验
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
	lv := u.Val("lv")
	exp := u.Val("exp")

	if newLv := Role.Upgrade.Submit(u, lv, exp); newLv != lv {
		role := u.Handle(config.ITypeRole)
		role.Add("lv", int32(newLv-lv))
	}
	//var newLv int32
	//
	//for i := lv + 1; ; i++ {
	//	if c := config.Data.Level[i]; c != nil && exp >= c.Exp {
	//		newLv = i
	//	} else {
	//		break
	//	}
	//}
	//
	//if newLv > 0 {
	//	u.Set(define.ItemTypeLV, newLv)
	//}

	return false
}
