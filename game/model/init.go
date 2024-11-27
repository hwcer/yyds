package model

import (
	"errors"
	"fmt"
	"github.com/hwcer/cosgo/redis"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/uuid"
	"runtime/debug"
	"server/share"
)

const BaseSize = 32

var DB = cosmo.New()
var UUID *uuid.Unique //随机自增种子,生成全服唯一ID
var Redis *redis.Client

type loading interface {
	init() error
}

type Player interface {
	Uid() uint64
	Unique(iid int32) *uuid.UUID
}

func GetUid(u *updater.Updater) uint64 {
	if p := GetPlayer(u); p != nil {
		return p.Uid()
	} else {
		logger.Alert("model.Uid player is nil:%v", string(debug.Stack()))
		return 0
	}
}

func GetPlayer(u *updater.Updater) Player {
	p, _ := u.Player.(Player)
	return p
}

// Unique 创建可以叠加的道具ID
func Unique(u *updater.Updater, iid int32) (string, error) {
	p := GetPlayer(u)
	if p == nil {
		return "", errors.New("player is not player")
	}
	return p.Unique(iid).String(BaseSize), nil
}

type Times struct {
	C int64 `json:"c" bson:"c"` //开始时间
	T int32 `json:"t" bson:"t"` //总需要秒数
	S int32 `json:"s" bson:"s"` //累计加速秒数
}

func (t *Times) Start(now int64, tot int32) {
	t.C = now
	t.T = tot
}

// Speed 加速
func (t *Times) Speed(now int64, val int32) bool {
	if t.Finish(now) {
		return false
	}
	t.S += val
	return true
}

// Remain 剩余时间
func (t *Times) Remain(now int64) int32 {
	n := int64(t.T - t.S)
	if now > 0 {
		n -= now - t.C
	}
	return int32(n)
}

// Finish 返回是否结束,以及结束时间
func (t *Times) Finish(now int64) (r bool) {
	if n := t.C + int64(t.T-t.S); n <= now {
		r = true
	}
	return
}

func (t *Times) Copy() *Times {
	r := *t
	return &r
}

// Register model注册之后服务器启动时自动检查创建索引
// 如果本身无索引，非必须
func Register(model interface{}) {
	DB.Register(model)
}

func Start() (err error) {
	sid := share.Options.Game.Sid
	if err = DB.Start(fmt.Sprintf("%v#S%v", share.AppId(), sid), share.Options.Game.Mongodb); err != nil {
		return
	}
	if share.Options.Game.Redis != "" {
		Redis, err = redis.New(share.Options.Game.Redis)
	}
	UUID = uuid.NewUnique(uint32(sid), BaseSize)
	updater.ITypes(func(k int32, it updater.IType) bool {
		if h, ok := it.(loading); ok {
			if err = h.init(); err != nil {
				return false
			}
		}
		return true
	})
	return
}

func Close() error {
	_ = DB.Close()
	if Redis != nil {
		_ = Redis.Close()
	}
	return nil
}

type Model struct {
	OID    string `bson:"_id" json:"id,omitempty"`
	IID    int32  `bson:"iid" json:"iid"`
	Uid    uint64 `bson:"uid" json:"-"  index:"name:_idx_uid_primary,Sort:asc,Priority:99" `
	Update int64  `bson:"update" json:"-"  index:"name:_idx_uid_primary,Sort:desc,Priority:100" ` //最后更新时间
}

func (this *Model) Init(u *updater.Updater, iid int32) {
	this.Uid = GetUid(u)
	this.IID = iid
	this.Update = u.Time.Unix()
}

func (this *Model) GetOID() string {
	return this.OID
}
func (this *Model) GetIID() int32 {
	return this.IID
}

func (this *Model) Clone() *Model {
	x := *this
	return &x
}

func (this *Model) Get(k string) (any, bool) {
	switch k {
	case "_id", "OID":
		return this.OID, true
	case "uid", "UID":
		return this.Uid, true
	case "iid", "IID":
		return this.IID, true
	case "update", "Update":
		return this.Update, true
	default:
		return nil, false
	}
}
func (this *Model) Set(k string, v any) (any, bool) {
	switch k {
	case "_id", "OID":
		this.OID = v.(string)
	case "uid", "UID":
		this.Uid = v.(uint64)
	case "iid", "IID":
		this.IID = dataset.ParseInt32(v)
	case "update", "Update":
		this.Update = dataset.ParseInt64(v)
	default:
		return v, false
	}
	return v, true
}

//func (this *Model) Saving(u dataset.Update) {
//	if _, ok := u["update"]; !ok {
//		u["update"] = time.Now()
//	}
//}

func (this *Model) SetOnInsert() (r map[string]interface{}, err error) {
	r = make(map[string]any)
	r["uid"] = this.Uid
	r["iid"] = this.IID
	return
}
