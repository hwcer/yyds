package player

import (
	"fmt"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/random"
	"github.com/hwcer/cosgo/slice"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/cosrpc/xclient"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players/emitter"
	"github.com/hwcer/yyds/players/verify"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Message struct {
	Id   int32  //req id
	Data []byte //*context.Message
}
type Handle func(*Player) error

func New(uid string) *Player {
	return &Player{uid: uid}
}

type Player struct {
	*updater.Updater
	uid       string
	mutex     sync.Mutex       //底层自动使用锁，不要手动调用
	heartbeat int64            //最后心跳时间
	Dirty     Dirty            //短连接推送数据缓存
	Login     int64            //登录时间
	Binder    binder.Binder    //当前端使用的序列化方式
	Status    int32            //在线状态
	Times     *Times           //时间控制器
	Verify    *verify.Verify   //全局条件验证
	Emitter   *emitter.Emitter //全局事件
	Message   *Message         //最后一次发包的 MESSAGE
	Gateway   uint64           //网关地址
}

func (p *Player) initialize() {
	if p.Times != nil {
		return
	}
	p.Times = &Times{p: p}
	p.Verify = verify.New(p.Updater)
	p.Emitter = emitter.New(p.Updater)
}

// Send 推送消息
// rp  req |  path
func (p *Player) Send(v any, rp any) {
	if p.Status != StatusConnected {
		logger.Debug("player disconnected:%v", p.Uid())
		return
	}
	if p.Gateway == 0 {
		logger.Debug("player gateway empty:%v", p.Uid())
		return
	}
	if p.Binder == nil {
		logger.Debug("player binder empty:%v", p.Uid())
		return
	}
	req := GetReqMeta(rp)
	if req == nil {
		return
	}
	guid := p.Guid()
	if guid == "" {
		logger.Debug("player gateway empty:%v", p.Uid())
		return
	}
	req.Set(binder.HeaderContentType, p.Binder.Name())
	req.Set(options.SelectorAddress, utils.IPv4Decode(p.Gateway))
	req.Set(options.ServiceMetadataUID, p.uid)
	req.Set(options.ServiceMetadataGUID, guid)
	_ = xclient.CallWithMetadata(req, nil, options.ServiceTypeGate, "send", v, nil)
}

// Loading 加载数据
// init 是否立即加载玩家数据，true:是
func (p *Player) Loading(init bool) (err error) {
	//验证UID是否合法
	if uid := p.Uid(); !uuid.IsValid(uid) {
		return fmt.Errorf("player uid(%s) is invalid", uid)
	}
	status := p.Status
	if status == StatusLocked || status == StatusRelease {
		return fmt.Errorf("player status disable")
	}
	if !atomic.CompareAndSwapInt32(&p.Status, status, StatusLocked) {
		return fmt.Errorf("player status change")
	}
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
			logger.Error(e)
		}
		if err != nil {
			p.Status = StatusRelease
		} else {
			p.Status = status
		}

	}()
	if p.Updater == nil {
		p.Updater = updater.New(p)
	}
	if err = p.Updater.Loading(init, p.initialize); err != nil {
		return err
	}
	return nil
}

func (p *Player) Uid() string {
	return p.uid
}

func (p *Player) Guid() string {
	doc := p.Document(ITypeRole)
	return doc.Get(Fields.Guid).(string)
}

func (p *Player) Destroy() error {
	if err := p.Updater.Destroy(); err != nil {
		return err
	}
	p.Updater = nil
	return nil
}
func (p *Player) On(t int32, args []int32, handle emitter.Handle) (r *emitter.Listener) {
	return p.Emitter.Listen(t, args, handle)
}
func (p *Player) Emit(t int32, v int32, args ...int32) {
	p.Emitter.Emit(t, v, args...)
}
func (p *Player) Listen(t int32, args []int32, handle emitter.Handle) (r *emitter.Listener) {
	return p.Emitter.Listen(t, args, handle)
}

func (p *Player) Heartbeat() int64 {
	return p.heartbeat
}

// KeepAlive 保持在线
func (p *Player) KeepAlive(t int64) {
	if t == 0 {
		if p.Updater != nil {
			t = p.Updater.Unix()
		} else {
			t = time.Now().Unix()
		}
	}

	p.heartbeat = t
}

// AddItems  无脑添加道具
// items类型itemGroup,itemProbability,[]itemGroup,[]itemProbability
// multi[分子,分母]
func (p *Player) AddItems(items interface{}, multi ...int32) {
	//概率
	power := [2]int32{1, 0}
	if len(multi) > 0 {
		copy(power[0:2], multi)
	}
	//独立概率
	if g, ok := items.(itemProbability); ok {
		if g.GetId() > 0 && g.GetNum() > 0 {
			var v int32
			for i := int32(0); i < power[0]; i++ {
				if random.Probability(g.GetVal()) {
					v += g.GetNum()
				}
			}
			if power[1] > 0 {
				v = v / power[1]
			}
			if v > 0 {
				p.Updater.Add(g.GetId(), v)
			}
		}
		return
	}
	//物品
	if g, ok := items.(itemGroup); ok {
		if g.GetId() > 0 && g.GetNum() > 0 {
			v := g.GetNum() * power[0]
			if power[1] > 0 {
				v = v / power[1]
			}
			p.Updater.Add(g.GetId(), v)
		}
		return
	}
	//概率组或者物品组
	vf := reflect.Indirect(reflect.ValueOf(items))
	if vf.Kind() == reflect.Slice || vf.Kind() == reflect.Array {
		for i := 0; i < vf.Len(); i++ {
			p.AddItems(vf.Index(i).Interface(), multi...)
		}
	}
}

// SubItems  无脑扣除道具
// items类型itemGroup,[]itemGroup
// multi[分子,分母]
func (p *Player) SubItems(items interface{}, multi ...int32) {
	//物品
	power := [2]int32{1, 0}
	if len(multi) > 0 {
		copy(power[0:2], multi)
	}
	if g, ok := items.(itemGroup); ok {
		if g.GetId() > 0 && g.GetNum() > 0 {
			v := g.GetNum() * power[0]
			if power[1] > 0 {
				v = v / power[1]
			}
			p.Updater.Sub(g.GetId(), v)
		}
		return
	}

	//概率组或者物品组
	vf := reflect.Indirect(reflect.ValueOf(items))
	if vf.Kind() == reflect.Slice || vf.Kind() == reflect.Array {
		for i := 0; i < vf.Len(); i++ {
			p.SubItems(vf.Index(i).Interface(), multi...)
		}
	}
}

func (p *Player) AddWithSlice(arr []int32) (r []int32) {
	for i := 0; i < len(arr); i += 2 {
		if j := i + 1; j < len(arr) {
			if arr[i] > 0 && arr[j] > 0 {
				r = append(r, arr[i])
				p.Add(arr[i], arr[j])
			}
		}
	}
	return
}

func (p *Player) AddWithString(s string, split string) (r []int32) {
	if s == "" || !strings.Contains(s, split) {
		return
	}
	ai := slice.SplitInt32(s, split)
	return p.AddWithSlice(ai)
}

func (p *Player) SubWithSlice(arr []int32) (r []int32) {
	for i := 0; i < len(arr); i += 2 {
		if j := i + 1; j < len(arr) {
			if arr[i] > 0 && arr[j] > 0 {
				r = append(r, arr[i])
				p.Sub(arr[i], arr[j])
			}
		}
	}
	return
}

func (p *Player) SubWithString(s string, split string) (r []int32) {
	if s == "" || !strings.Contains(s, split) {
		return
	}
	ai := slice.SplitInt32(s, split)
	return p.SubWithSlice(ai)
}
