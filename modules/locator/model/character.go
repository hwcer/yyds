package model

import (
	"fmt"

	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
)

func init() {
	db.Register(&Character{})
}

type Character struct {
	Uid    string        `json:"uid" bson:"_id"`
	Sid    int32         `json:"sid" bson:"sid"`                  //服务器ID
	Guid   string        `bson:"guid" json:"guid" index:""`       //账号ID
	Online int64         `json:"online" bson:"online"`            //最后登陆时间
	Update int64         `json:"update,omitempty" bson:"update" ` //最后更新时间
	Create int64         `json:"create,omitempty" bson:"create" ` //创建时间
	Attach values.Values `json:"attach,omitempty" bson:"attach" ` // lv name ...
}

func (this *Character) SetAttach(k string, v any) {
	if this.Attach == nil {
		this.Attach = values.Values{}
	}
	this.Attach.Set(k, v)
}

func (this *Character) GetUpdate() map[string]any {
	r := make(map[string]any)
	r["update"] = times.Now().Unix()
	for k, v := range this.Attach {
		rk := fmt.Sprintf("attach.%v", k)
		r[rk] = v
	}

	return r
}
