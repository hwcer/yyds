package player

import (
	"fmt"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/kernel/config"
	"strings"
)

type Role struct {
	*updater.Document
}

func NewRole(p *Player) *Role {
	doc := p.Document(config.ITypeRole)
	r := &Role{Document: doc}
	return r
}

func (this *Role) rk(k string, fields ...any) string {
	if len(fields) > 0 {
		arr := []string{k}
		for _, i := range fields {
			arr = append(arr, fmt.Sprintf("%v", i))
		}
		k = strings.Join(arr, ".")
	}
	return k
}

func (this *Role) Set(k string, v any, fields ...any) {
	if len(fields) > 0 {
		k = this.rk(k, fields...)
	}
	this.Document.Set(k, v)
}
func (this *Role) Get(k string, fields ...any) any {
	if len(fields) > 0 {
		k = this.rk(k, fields...)
	}
	return this.Document.Get(k)
}

func (this *Role) Add(k string, v int32, fields ...any) {
	if len(fields) > 0 {
		k = this.rk(k, fields...)
	}
	this.Document.Add(k, v)
}
func (this *Role) Sub(k string, v int32, fields ...any) {
	if len(fields) > 0 {
		k = this.rk(k, fields...)
	}
	this.Document.Sub(k, v)
}

//func (this *Role) All() *model.Role {
//	return this.Document.Any().(*model.Role)
//}

//func (this *Role) GetDaily() (*model.RoleDaily, error) {
//	r := this.All()
//	if err := r.Daily.Verify(this.Updater); err != nil {
//		return nil, err
//	}
//	return &r.Daily, nil
//}
//
//func (this *Role) SetDaily(k string, v any) {
//	arr := []any{model.RoleHandleDailyVal, k}
//	rk := this.rk(model.RoleHandleDailyName, arr...)
//	this.Document.Set(rk, v)
//}
