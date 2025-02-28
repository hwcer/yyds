package player

import (
	"fmt"
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
	"sync/atomic"
)

func (p *Player) Send(path string, v any, req values.Metadata) {
	if p.Status != StatusConnected {
		logger.Debug("player disconnected:%v", p.Uid())
		return
	}
	if p.Gateway == 0 {
		logger.Debug("player gateway empty:%v", p.Uid())
		return
	}
	if req == nil {
		req = values.Metadata{}
	}

	role := p.Document(options.ITypeRole)
	guid := role.Get(Fields.Guid)
	if guid == nil {
		logger.Debug("player gateway empty:%v", p.Uid())
		return
	}
	req.Set(options.ServiceMetadataGUID, guid)
	req.Set(options.ServiceMessagePath, path)
	req.Set(options.ServiceMetadataRequestId, utils.IPv4Decode(p.Gateway))

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

func (p *Player) Destroy() error {
	if err := p.Updater.Destroy(); err != nil {
		return err
	}
	p.Updater = nil
	return nil
}

func (p *Player) Emit(t int32, v int32, args ...int32) {
	c := GetEmitter(t)
	if c == nil {
		return
	}
	if c.Daily > 0 {
		p.EventUpdate(c.Daily, v, c.Replace)
	}
	if c.Record > 0 {
		p.EventUpdate(c.Record, v, c.Replace)
	}
	if c.Event > 0 {
		p.Emitter.Emit(c.Event, v, args...)
	}
}
func (p *Player) Listen(t int32, args []int32, handle emitter.Handle) (r *emitter.Listener) {
	return p.Emitter.Listen(t, args, handle)
}

func (p *Player) EventUpdate(k int32, v int32, replace int32) {
	if replace != 0 {
		p.Updater.Set(k, v)
	} else {
		p.Updater.Add(k, v)
	}
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

func (p *Player) AddArr(arr []int32) {
	for i := 0; i < len(arr); i += 2 {
		j := i + 1
		if j < len(arr) {
			if arr[i] > 0 && arr[j] > 0 {
				p.Add(arr[i], arr[j])
			}
		}
	}
}

func (p *Player) SubArr(arr []int32) {
	for i := 0; i < len(arr); i += 2 {
		j := i + 1
		if j < len(arr) {
			if arr[i] > 0 && arr[j] > 0 {
				p.Sub(arr[i], arr[j])
			}
		}
	}
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
