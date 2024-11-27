package model

import (
	"fmt"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/game/config"
	"strconv"
	"strings"
)

type MailStatus int8

const (
	MailStatusNone   MailStatus = 0 //新邮件
	MailStatusRead              = 1 //已读，但有附件未领
	MailStatusFinal             = 2 //已读，已领，可删
	MailStatusDelete            = 9 //已删除
)

func init() {
	Register(&Mail{})
}

func NewMail(u *updater.Updater, iid int32, args map[string]any) *Mail {
	m := &Mail{Args: args}
	m.Init(u, iid)
	return m
}

type Mail struct {
	Model  `bson:"inline"`
	Attr   []MailAttr     `json:"attr" bson:"attr"`     //附件
	Args   map[string]any `json:"args" bson:"args"`     //系统邮件参数
	Text   *MailText      `json:"text" bson:"text"`     //多语言文本
	Status MailStatus     `json:"status" bson:"status"` //邮件状态
}

type MailAttr struct {
	K int32 `json:"k" bson:"k"`
	V int32 `json:"v" bson:"v"`
}

//type MailItems []*MailAttr

type MailText struct {
	From    string `json:"from" bson:"from"`       //发送者
	Title   string `json:"title" json:"title"`     //标题
	Content string `json:"content" json:"content"` //内容
}

func (this *MailAttr) GetId() int32 {
	return this.K
}

func (this *MailAttr) GetNum() int32 {
	return this.V
}

func (this *Mail) Clone(uid uint64, xid any) *Mail {
	t := *this
	m := &t
	if len(this.Attr) > 0 {
		m.Attr = make([]MailAttr, 0, len(this.Attr))
		m.Attr = append(m.Attr, this.Attr...)
	} else {
		m.Attr = []MailAttr{}
	}

	m.Args = make(map[string]any)
	for k, v := range this.Args {
		m.Args[k] = v
	}
	m.Uid = uid
	m.ObjectId(xid)
	return m
}

func (this *Mail) AddArgs(k string, v interface{}) {
	if this.Args == nil {
		this.Args = make(map[string]interface{})
	}
	this.Args[k] = v
}

func (this *Mail) AddAttr(k, v int32) {
	if k == 0 || v == 0 {
		return
	}
	if it := config.GetIType(k); it == 0 {
		return
	}
	this.Attr = append(this.Attr, MailAttr{K: k, V: v})
}

// ObjectId xid 竞技场赛季ID,活动ID等 避免不同活动 使用相同模板时重复
// iid 模板ID
func (this *Mail) ObjectId(xid any) {
	this.OID = fmt.Sprintf("%vx%v-%v", this.Uid, this.IID, xid)
}

// AddItemFromInput 附件 "11002,100,15001...."
func (this *Mail) AddItemFromInput(attr string) error {
	if attr == "" {
		return nil
	}
	arr := strings.Split(attr, ",")
	var i, j int
	var n = len(arr)
	for i = 0; i < n; i += 2 {
		j = i + 1
		if j >= n {
			return values.Errorf(0, "items error") //输入错误
		}
		k, err := strconv.Atoi(arr[i])
		if err != nil {
			return fmt.Errorf("attr error:%v", arr[i])
		}
		v, err := strconv.Atoi(arr[j])
		if err != nil {
			return fmt.Errorf("attr error:%v", arr[j])
		}
		this.AddAttr(int32(k), int32(v))
	}
	return nil
}
