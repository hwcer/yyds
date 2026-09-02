package player

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
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
	"github.com/hwcer/yyds/players/condition"
	"github.com/hwcer/yyds/players/emitter"
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

// Player 玩家对象
//
// # 加锁契约
//
// 除下列方法外,**所有导出方法(含从 *updater.Updater 提升上来的 Add/Sub/Set/Get/Val/
// Select/Data/Submit/Verify/Document/Collection 等)都必须在持有玩家锁时调用**。
// 它们会读写 Updater 及其 dataset,而 released() 会在锁内把 Updater 置空:
// 不持锁轻则读到残值,重则撞上 dataset 里 map 的并发读写 —— 那是 fatal error,
// recover 拦不住,直接打挂进程。
//
// 无需持锁的(只碰不可变字段或原子字段):
//
//	Uid / Key                 创建后不再变
//	Status 相关: Denied / Connected / Terminate
//	心跳:       Heartbeat / KeepAlive
//	锁本身:     Lock / Unlock
//
// 例外:对象尚未进入 Manage(其他协程拿不到指针)时不必加锁,预加载
// (players/preload.go)就是先 Loading 再 Store。
type Player struct {
	*updater.Updater
	uid       string
	key       string              //map key，空值时 Key() 返回 uid
	heartbeat atomic.Int64        //最后心跳时间,daemon 与业务协程并发读写,必须原子操作
	Pending   Pending             //待发送的 Operator 暂存区(跨玩家场景让出锁前 Submit 出来的)
	Login     int64               //登录时间
	mutex     sync.Mutex          //玩家锁,见类型注释的加锁契约;经 Lock/Unlock 访问
	Binder    binder.Binder       //当前端使用的序列化方式
	Status    int32               //在线状态
	Times     *Times              //时间控制器
	Condition *condition.Verifier //全局条件验证器
	Emitter   *emitter.Emitter    //全局事件
	Message   *Message            //最后一次发包的 MESSAGE
	Gateway   uint64              //网关地址
	Address   string              //客户端地址(ip)
}

func (p *Player) createComponents() {
	if p.Times != nil {
		return
	}
	p.Times = &Times{p: p}
	p.Condition = condition.New(p.Updater)
	p.Emitter = emitter.New(p.Updater)
}

// Send 推送消息
// rp  req |  path
//
// 调用方必须持有玩家锁。开头那次状态判定是锁外快筛,后面读 Binder/Gateway 和调 Guid()
// (要碰 Updater 的 dataset)都没有复查:不持锁时 released() 可能刚好在这中间跑完 Destroy,
// 读到的是残值,并发改 dataset 更可能撞上 concurrent map read and write 这种 fatal error。
// 目前调用方都在 players.Get 回调里(context.Send / master.Send),天然满足;
// 若要从 daemon、定时器或别的玩家协程里推消息,请套 players.Get(uid, func(p){ p.Send(...) })。
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

	//网关按 socketId 与 GUID 二选一定位连接,两者都空才是真的投不出去。
	//Guid 取不到通常意味着角色数据不可读(见 Guid 的说明),但只要请求带了 SocketId 仍能投递
	guid := p.Guid()
	if guid == "" && req[gwcfg.ServiceMetadataSocketId] == "" {
		logger.Debug("player guid empty and no socket id:%s", p.Uid())
		return
	}
	if _, ok := req[binder.HeaderContentType]; !ok {
		req.Set(binder.HeaderContentType, p.Binder.Name())
	}
	req.Set(selector.MetaDataAddress, utils.IPv4Decode(p.Gateway))
	req.Set(gwcfg.ServiceMetadataUID, p.uid)
	if guid != "" {
		req.Set(gwcfg.ServiceMetadataGUID, guid) //空值别写进去,免得网关拿它当有效标识
	}
	if err := client.CallWithMetadata(req, nil, gwcfg.ServiceTypeGate, gwcfg.MessageSend, v, nil); err != nil {
		//不能吞掉:推送失败在客户端表现为"没收到",服务端不留痕就完全查不出来
		logger.Error("消息推送失败,uid:%v,guid:%v,path:%v,err:%v", p.uid, guid, req[gwcfg.ServiceMessagePath], err)
	}
}

// Loading 加载数据
// test 是否测试模式
func (p *Player) Loading(test bool) (err error) {
	//验证UID是否合法
	if uid := p.Uid(); !uuid.IsValid(uid) {
		return fmt.Errorf("player uid(%s) is invalid", uid)
	}
	status := atomic.LoadInt32(&p.Status)
	if p.Denied(status) {
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
	if err = p.Updater.Loading(p.createComponents); err != nil {
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

// Guid 角色的全局唯一标识,取不到时返回空串,调用方按空串处理(参考 Send)
// 不做裸类型断言:Updater 已销毁(released)、handle 未注册、字段未加载都会让上一版直接 panic,
// 而本函数在 gomcp / Send 等路径上会被非业务协程调用
func (p *Player) Guid() string {
	if p.Updater == nil {
		return ""
	}
	doc := p.Document(RoleIType)
	if doc == nil {
		return ""
	}
	guid, _ := doc.Get(RoleFields.Guid).(string)
	return guid
}

// Destroy 销毁玩家数据,调用者必须持有玩家锁(p.Lock),否则会与在途业务调用竞态置空 Updater
func (p *Player) Destroy() error {
	//空壳(Updater==nil,见 players.Lock)没有数据要销毁,直接算成功。
	//不能返回 error:daemon 的 released 收到 error 会还原状态、下个 tick 重试,
	//空壳每次都失败就成了死循环。
	if p.Updater == nil {
		p.Pending = Pending{}
		p.Emitter = nil
		return nil
	}
	if err := p.Updater.Destroy(); err != nil {
		return err
	}
	p.Updater = nil
	p.Pending = Pending{}
	p.Emitter = nil
	return nil
}

// Initialize 把玩家初始化到可用状态,供 Load(init=false) 的回调「中途发现需要数据」时调用。
//
// 它把 Loading 与 Reset 一次做完 —— 单调 Loading 是不够的:Updater 由 New 创建时
// now 是零值,只有 Reset 会设,漏掉就得到一个"能用但时间基准是 1 年"的 Updater,
// 不报错也不 panic,是最难查的那类问题。
//
// 幂等:Updater 已就绪(init=true,或 init=false 时玩家本就在内存、框架已 Reset 过)
// 直接返回。Release 由框架的 defer 负责,调用方不必管。
func (p *Player) Initialize() error {
	if p.Updater != nil {
		return nil
	}
	if err := p.Loading(false); err != nil {
		return err
	}
	p.Reset()
	return nil
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
