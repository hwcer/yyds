package model

import (
	"fmt"
)

func init() {
	db.Register(&Analyse{})
}

var DAU = make(map[int32]struct{})

func init() {
	DAU[2] = struct{}{}
	DAU[3] = struct{}{}
	DAU[4] = struct{}{}
	DAU[5] = struct{}{}
	DAU[6] = struct{}{}
	DAU[7] = struct{}{}
	DAU[14] = struct{}{}
	DAU[15] = struct{}{}
	DAU[30] = struct{}{}

}

type Analyse struct {
	Id     string          `json:"id" bson:"_id"` //sid+day
	Sid    int32           `json:"sid" bson:"sid" index:"" `
	Day    int32           `json:"day" bson:"day" index:"" `
	Create int32           `json:"create" bson:"create"` //新用户
	Active map[int32]int32 `json:"active" bson:"active"` //每日活跃用户
}

func NewAnalyse(sid, day int32) *Analyse {
	r := &Analyse{Sid: sid, Day: day}
	r.Id = fmt.Sprintf("%v-%v", sid, day)
	r.Active = make(map[int32]int32)
	return r
}

func (this *Analyse) SetOnInsert() (map[string]any, error) {
	r := make(map[string]any)
	r["sid"] = this.Sid
	r["day"] = this.Day
	return r, nil
}
