package player

import (
	"fmt"
	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/random"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/xclient"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players/emitter"
	"reflect"
	"strings"
	"sync/atomic"
)

func GetReqMeta(rp any) (req values.Metadata) {
	switch t := rp.(type) {
	case string:
		req = values.Metadata{}
		req.Set(options.ServiceMessagePath, t)
	case map[string]string:
		req = t
	case values.Metadata:
		req = t
	default:
		logger.Alert("unknown req type %v", reflect.TypeOf(rp))
		return
	}
	if _, ok := req[options.ServiceMessagePath]; !ok {
		logger.Alert("req no service message path:%v", rp)
		return
	}
	return
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
	status := p.Status
	if status == StatusLocked || status == StatusRelease {
		return fmt.Errorf("player status disable")
	}
	if !atomic.CompareAndSwapInt32(&p.Status, status, StatusLocked) {
		return fmt.Errorf("player status change")
	}
	defer func() {
		if err != nil {
			p.Status = StatusRelease
		} else {
			p.Status = status
		}
	}()
	if p.Updater == nil {
		p.Updater = updater.New(p.uid)
		p.Updater.Process.Set(ProcessName, p)
	}
	if err = p.Updater.Loading(init, p.initialize); err != nil {
		return err
	}
	return nil
}

func (p *Player) Uid() uint64 {
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
			t = times.Unix()
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
	if s == "" || !strings.Contains(split, split) {
		return
	}
	as := strings.Split(s, split)
	ai := utils.SliceStringToInt32(as)
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
	if s == "" || !strings.Contains(split, split) {
		return
	}
	as := strings.Split(s, split)
	ai := utils.SliceStringToInt32(as)
	return p.SubWithSlice(ai)
}

// MustUpdate 客户端数据是否需要更新
// -1 : 不需要强制更新
// 0 : 强制更新
// >0:开始更新的时间节点
//func (p *Player) MustUpdate() int64 {
//	if !p.mustUpdate {
//		return -1
//	} else {
//		return p.lastUpdate
//	}
//}

func (p *Player) Values(name any) *updater.Values {
	i := p.Updater.Handle(name)
	if i == nil {
		return nil
	}
	r, _ := i.(*updater.Values)
	return r
}
func (p *Player) Document(name any) *updater.Document {
	i := p.Updater.Handle(name)
	if i == nil {
		return nil
	}
	r, _ := i.(*updater.Document)
	return r
}

func (p *Player) Collection(name any) *updater.Collection {
	i := p.Updater.Handle(name)
	if i == nil {
		return nil
	}
	r, _ := i.(*updater.Collection)
	return r
}
