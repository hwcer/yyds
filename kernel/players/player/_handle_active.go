package player

import (
	"errors"
	"fmt"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/emitter"
	"github.com/hwcer/yyds/kernel/model"
	"strings"
)

const ActiveListenerKey = "active_listener_key"

var ActiveModel = sActiveModel{}

type sActiveModel map[int32]ActiveHandle

func (this sActiveModel) Register(k int32, h ActiveHandle) {
	this[k] = h
}

type ActiveHandle interface {
	Init(*Player, *model.Active)
	Handle(*Player, ActiveListener, int32) bool
}

type ActiveListener interface {
	GetId() int32
	GetKey() int32
	GetArgs() []int32
}

type Active struct {
	*updater.Collection
	player *Player
}

func NewActive(p *Player) *Active {
	doc := p.Collection(define.ITypeActive)
	r := &Active{Collection: doc, player: p}
	doc.Range(func(id string, doc *dataset.Document) bool {
		v, _ := doc.Any().(*model.Active)
		c := config.Data.Active[v.IID]
		if c == nil {
			return true
		}
		if m := ActiveModel[c.Mod]; m != nil {
			m.Init(p, v)
		}
		return true
	})
	return r
}
func (this *Active) rk(k string, fields ...any) string {
	if len(fields) > 0 {
		arr := []string{k}
		for _, i := range fields {
			arr = append(arr, fmt.Sprintf("%v", i))
		}
		k = strings.Join(arr, ".")
	}
	return k
}
func (this *Active) Get(id int32) (r *model.Active) {
	if i := this.Collection.Get(id); i != nil {
		v, _ := i.(*model.Active)
		r = v.Copy()
	}
	return
}

func (this *Active) Listener(al ActiveListener) *emitter.Listener {
	c := config.Data.Active[al.GetId()]
	if c == nil {
		return nil
	}
	if _, ok := ActiveModel[c.Mod]; !ok {
		return nil
	}
	if _, e := this.player.Expire.Verify(c.Times); e != nil {
		return nil
	}
	l := this.player.Emitter.Listener(al.GetKey(), al.GetArgs(), this.handle)
	l.Attach.Set(ActiveListenerKey, al)
	return l
}

func (this *Active) handle(l *emitter.Listener, val int32) bool {
	al, ok := l.Attach.Get(ActiveListenerKey).(ActiveListener)
	if !ok {
		return false
	}
	c := config.Data.Active[al.GetId()]
	if c == nil {
		return false
	}
	m := ActiveModel[c.Mod]
	if m == nil {
		return false
	}
	return m.Handle(this.player, al, val)
}

func (this *Active) Verify(p *Player, id int32, limit []int64) (r *model.Active, t [2]int64, err error) {
	cfg := config.Data.Active[id]
	if cfg == nil {
		err = errors.New("invalid active id")
		return
	}
	//开启时间检查
	if t, err = this.player.Expire.Verify(limit); err != nil {
		return
	}
	//WEEKS
	if len(cfg.Weeks) > 0 && !Arr(cfg.Weeks).Has(int32(p.Time.Weekday())) {
		err = errors.New("week limit")
		return
	}

	this.Select(id)
	if err = p.Data(); err != nil {
		return
	}
	r = this.Get(id)
	if r == nil {
		if r, err = model.ITypeActive.Create(p.Updater, id, 0); err == nil {
			err = p.Create(r)
		}
		if err != nil {
			return
		}
	}
	if cfg.AType != config.Data.ActiveType.None && r.Expire < p.Time.Unix() {
		var ttl *times.Times
		if ttl, err = times.Expire(times.ExpireType(cfg.AType), 1); err == nil {
			return
		}
		r.Expire = ttl.Unix()
		r.Attach = values.Values{}
		this.Set(r.OID, "att", r.Attach)
		this.Set(r.OID, "ttl", r.Expire)
	}
	return
}
func (this *Active) Update(id string, k string, v any, fields ...any) {
	if len(fields) > 0 {
		k = this.rk(k, fields...)
	}
	this.Set(id, k, v)
}

func (this *Active) SetAttach(id string, k string, v any) {
	this.Update(id, "att", v, k)
}
