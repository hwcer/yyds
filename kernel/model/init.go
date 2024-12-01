package model

import (
	"errors"
	"fmt"
	"github.com/hwcer/cosgo/options"
	"github.com/hwcer/cosgo/redis"
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/yyds/kernel/config"
)

const BaseSize = 32
const UpdaterProcessBuilder = "_player_builder"

var DB = cosmo.New()
var Redis *redis.Client
var Builder *uuid.Unique //随机自增种子,生成全服唯一ID

type initialize interface {
	init() error
}

// Unique 创建可以叠加的道具ID
func Unique(u *updater.Updater, iid int32) (r string, err error) {
	var b *uuid.Builder
	if i := u.Process.Get(UpdaterProcessBuilder); i == nil {
		if b, err = uuid.Create(u.Uid(), BaseSize); err == nil {
			u.Process.Set(UpdaterProcessBuilder, b)
		}
	} else {
		b = i.(*uuid.Builder)
	}
	if b != nil {
		r = b.New(uint32(iid)).String(BaseSize)
	} else if err == nil {
		err = errors.New("Updater.Process player builder error")
	}
	return
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
	sid := options.Game.Sid
	if err = DB.Start(fmt.Sprintf("%v#S%v", options.Options.Appid, sid), options.Game.Mongodb); err != nil {
		return
	}
	if options.Game.Redis != "" {
		Redis, err = redis.New(options.Game.Redis)
	}
	Builder = uuid.NewUnique(uint32(sid), BaseSize)
	updater.ITypes(func(k int32, it updater.IType) bool {
		if h, ok := it.(initialize); ok {
			if err = h.init(); err != nil {
				return false
			}
		}
		return true
	})
	updater.Models(func(k int32, m any) bool {
		if h, ok := m.(initialize); ok {
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
	this.Uid, _ = u.Uid().(uint64)
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
func (this *Model) IType(id int32) int32 {
	return config.GetIType(id)
}
func (this *Model) SetOnInsert() (r map[string]interface{}, err error) {
	r = make(map[string]any)
	r["uid"] = this.Uid
	r["iid"] = this.IID
	return
}
