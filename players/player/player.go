package player

import (
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hwcer/cosgo/binder"
	"github.com/hwcer/cosgo/random"
	"github.com/hwcer/cosgo/slice"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/uuid"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/cosrpc/selector"
	"github.com/hwcer/gateway/gwcfg"
	"github.com/hwcer/logger"
	"github.com/hwcer/updater"
	"github.com/hwcer/yyds/players/emitter"
	"github.com/hwcer/yyds/players/verify"
)

type Message struct {
	Id   int32  //req id
	Data []byte //*context.Message
}
type Handle func(*Player) error

func New(uid string, test bool) *Player {
	p := &Player{uid: uid}
	if test {
		p.key = "~" + uid
	}
	return p
}

type Player struct {
	*updater.Updater
	uid       string
	key       string           //map key，空值时 Key() 返回 uid
	heartbeat atomic.Int64     //最后心跳时间,daemon 与业务协程并发读写,必须原子操作
	Dirty     Dirty            //短连接推送数据缓存
	Login     int64            //登录时间
	Syncer    Syncer           //并发控制器
	Binder    binder.Binder    //当前端使用的序列化方式
	Status    int32            //在线状态
	Times     *Times           //时间控制器
	Verify    *verify.Verify   //全局条件验证
	Emitter   *emitter.Emitter //全局事件
	Message   *Message         //最后一次发包的 MESSAGE
	Gateway   uint64           //网关地址
	ClientIp  string           //客户端IP
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
func (p *Player) Send(v any, req values.Metadata) {
	if atomic.LoadInt32(&p.Status) != StatusConnected {
		logger.Debug("player disconnected:%s", p.Uid())
		return
	}
	if p.Gateway == 0 {
		logger.Debug("player gateway empty:%s", p.Uid())
		return
	}
	if p.Binder == nil {
		logger.Debug("player binder empty:%s", p.Uid())
		return
	}
	if req == nil {
		return
	}
	if _, ok := req[gwcfg.ServiceMessagePath]; !ok {
		logger.Debug("player send path empty")
		return
	}

	guid := p.Guid()
	if guid == "" {
		logger.Debug("player gateway empty:%s", p.Uid())
		return
	}
	if _, ok := req[binder.HeaderContentType]; !ok {
		req.Set(binder.HeaderContentType, p.Binder.Name())
	}
	req.Set(selector.MetaDataAddress, utils.IPv4Decode(p.Gateway))
	req.Set(gwcfg.ServiceMetadataUID, p.uid)
	req.Set(gwcfg.ServiceMetadataGUID, guid)
	_ = client.CallWithMetadata(req, nil, gwcfg.ServiceName, gwcfg.MessageSend, v, nil)
}

// Loading 加载数据
// test 是否测试模式
func (p *Player) Loading(test bool) (err error) {
	//验证UID是否合法
	if uid := p.Uid(); !uuid.IsValid(uid) {
		return fmt.Errorf("player uid(%s) is invalid", uid)
	}

	status := atomic.LoadInt32(&p.Status)
	if status == StatusLocked || status == StatusReleased {
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
			atomic.StoreInt32(&p.Status, StatusReleased)
		} else {
			atomic.StoreInt32(&p.Status, status)
		}
	}()
	if p.Updater == nil {
		p.Updater = updater.New(p)
	}
	if err = p.Updater.Loading(p.initialize); err != nil {
		return err
	}
	if test {
		err = p.Updater.Testing(true)
	}
	return
}

func (p *Player) Key() string {
	if p.key != "" {
		return p.key
	}
	return p.uid
}

func (p *Player) Uid() string {
	return p.uid
}

func (p *Player) Guid() string {
	doc := p.Document(RoleIType)
	return doc.Get(RoleFields.Guid).(string)
}

// Destroy 销毁玩家数据,调用者必须持有玩家锁(p.Lock),否则会与在途业务调用竞态置空 Updater
// 不在这里关闭 Syncer:关闭动作必须发生在 Unlock 之后,见 Close
func (p *Player) Destroy() error {
	if err := p.Updater.Destroy(); err != nil {
		return err
	}
	p.Updater = nil
	p.Dirty = Dirty{}
	p.Emitter = nil
	return nil
}

// Close 关闭并发控制器,必须在 Unlock 之后调用
// actor 模式下 Syncer.Close 会关掉玩家通道,持锁时关闭会让通道 worker 永久阻塞在 nil channel 上
func (p *Player) Close() {
	if p.Syncer != nil {
		p.Syncer.Close()
	}
}
func (p *Player) On(t int32, args []int32, handle emitter.Callback) (r *emitter.Context) {
	return p.Emitter.On(t, args, handle)
}
func (p *Player) Emit(t int32, v int32, args ...int32) {
	p.Emitter.Emit(t, v, args...)
}

// Listen 使用name注册监听避免重复,同名覆盖参数和回调
func (p *Player) Listen(name string, t int32, args []int32, handle emitter.Listener) (r *emitter.Context, err error) {
	return p.Emitter.Listen(name, t, args, handle)
}
func (p *Player) Connected() bool {
	return atomic.LoadInt32(&p.Status) == StatusConnected
}

// Heartbeat 获取最后心跳时间
func (p *Player) Heartbeat() int64 {
	return p.heartbeat.Load()
}

// KeepAlive 保持在线,t 为 0 时取当前时间
//
// 注意:t==0 时不能退化到 Updater.Unix(),那是"本次请求的开始时间",只在 Reset() 时刷新。
// 玩家空闲(不再发包)时该值冻结在最后一次请求上,daemon 里 disconnect/offline
// 想借 KeepAlive(0) 重置状态机计时就会失效,导致 Connected→Disconnect→Offline
// 在相邻几个 tick 内一路走完,120s 的断线重连宽限期形同虚设。
func (p *Player) KeepAlive(t int64) {
	if t == 0 {
		t = time.Now().Unix()
	}
	p.heartbeat.Store(t)
}

// AddItems  无脑添加道具
// items类型itemGroup,itemProbability,[]itemGroup,[]itemProbability
// multi[分子,分母]
func (p *Player) AddItems(items interface{}, multi ...int64) {
	//概率
	power := [2]int64{1, 0}
	if len(multi) > 0 {
		copy(power[0:2], multi)
	}
	//独立概率
	if g, ok := items.(itemProbability); ok {
		if g.GetId() > 0 && g.GetNum() > 0 {
			var v int64
			for i := int64(0); i < power[0]; i++ {
				if random.Probability(g.GetVal()) {
					v += int64(g.GetNum())
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
			v := int64(g.GetNum()) * power[0]
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
func (p *Player) SubItems(items interface{}, multi ...int64) {
	//物品
	power := [2]int64{1, 0}
	if len(multi) > 0 {
		copy(power[0:2], multi)
	}
	if g, ok := items.(itemGroup); ok {
		if g.GetId() > 0 && g.GetNum() > 0 {
			v := int64(g.GetNum()) * power[0]
			if power[1] > 0 {
				v = v / power[1]
			}
			if v <= 0 {
				logger.Alert("sub items error, uid:%s,iid:%d,val:%d,num:%d", p.Uid(), g.GetId(), v, power[0])
				_ = p.Updater.Errorf("sub items is invalid")
				return
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
