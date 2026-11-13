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
	"github.com/hwcer/yyds/options"
	"github.com/hwcer/yyds/players/condition"
	"github.com/hwcer/yyds/players/emitter"
	"github.com/hwcer/yyds/players/inbox"
)

type Message struct {
	Id   int32  //req id
	Data []byte //*context.Message
}
type Handle func(*Player) error

type CallerHandle func(*Player, ...any) error

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
	Emitter   *emitter.Emitter    //全局事件（业务）
	Inbox     *inbox.Inbox        //系统事件收件箱（见 players/inbox）
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
	p.Inbox = inbox.New(p.Updater)
}

// Send 推送消息
// rp  req |  path
//
// 调用方必须持有玩家锁。开头那次状态判定是锁外快筛,后面读 Binder/Gateway 和调 Guid()
// (要碰 Updater 的 dataset)都没有复查:不持锁时 released() 可能刚好在这中间跑完 Unload,
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

	//网关按 socketId > UID 定位推送目标:请求驱动的推送认 socketId(回到发起它的那条连接,
	//代次隔离);没有 socketId 的(定时器/跨玩家等主动推送)由网关按 UID 直查会话表
	//(表键就是 uid)。GUID 只做认证,不参与定位。
	//uid 恒非空(Player 构造即有),此守卫只是防御,正常永远走不到
	guid := p.Guid()
	if req[gwcfg.ServiceMetadataSocketId] == "" && p.uid == "" {
		logger.Debug("player uid and socket id both empty:%s", p.Uid())
		return
	}
	if _, ok := req[binder.HeaderContentType]; !ok {
		req.Set(binder.HeaderContentType, p.Binder.Name())
	}
	req.Set(selector.MetaDataAddress, utils.IPv4Decode(p.Gateway))
	req.Set(gwcfg.ServiceMetadataUID, p.uid)
	if guid != "" {
		//随行身份信息:网关 send 不消费它(定位靠 socketId/UID),留给日志与业务钩子
		req.Set(gwcfg.ServiceMetadataGUID, guid)
	}
	if err := client.CallWithMetadata(req, nil, gwcfg.ServiceTypeGate, gwcfg.MessageSend, v, nil); err != nil {
		//不能吞掉:推送失败在客户端表现为"没收到",服务端不留痕就完全查不出来
		logger.Error("消息推送失败,uid:%v,guid:%v,path:%v,err:%v", p.uid, guid, req[gwcfg.ServiceMessagePath], err)
	}
}

// Loading 加载数据,调用方负责随后 Reset;要测试模式(只改内存不写库)的,加载后自己调
// p.Updater.Testing(true)。
//
//	lazy  懒加载:不预拉任何数据(updater 懒加载),供批量锁给不在内存的玩家建 Updater。
//	      读之前必须 Select + Data(没拉的键读到零值),扫描类接口报错,契约见 player.Locker
//	      与 updater 仓 CLAUDE.md「懒加载」。已整档加载的实例传 true 无效;懒加载的实例再以
//	      lazy=false 调用即转正(整档预拉)
func (p *Player) Loading(lazy bool) (err error) {
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
	//🔴 失败后的状态分两种:新建 Updater 失败 / panic → Released(拒绝态),由调用方摘出 Manage
	//(players.load、batch.loading);已有 Updater 的转正失败(如批量锁里对懒加载玩家 Loading(false))
	//→ 恢复原状态——updater 已退回懒加载,对象仍可用。打成 Released 而调用方又摘不掉(业务侧
	//拿不到 Manage)的话,daemon 不回收拒绝态,该玩家此后进不了任何批量锁、登录也要先失败一次。
	fresh := p.Updater == nil
	panicked := false
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
			panicked = true
			logger.Error(e)
		}
		if err != nil && (fresh || panicked) {
			atomic.StoreInt32(&p.Status, StatusReleased)
		} else {
			atomic.StoreInt32(&p.Status, status)
		}
	}()
	if fresh {
		//玩家域即 Default 域;包级 New() 现在是造域(返回 *Manage),实例经 Manage.New(e) 取
		p.Updater = updater.Default.New(p)
	}
	//组件建立先于首次 Loading（updater.Loading 已不带回调）：Emit(EventTypeInit) 时组件
	//就绪，与旧回调形态的时序等价。无条件调用是安全的——createComponents 自身幂等
	//(Times 已建即返回)，自愈路径与懒加载重拉起都不重复建、不丢已注册监听
	p.createComponents()
	_, err = p.Updater.Loading(lazy)
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

// Id 数据属主标识,实现 updater.Entity（Updater.Entity 重构后 New(e) 的准入接口）
func (p *Player) Id() string {
	return p.uid
}

// Guid 角色的全局唯一标识,取不到时返回空串,调用方按空串处理(参考 Send)。
// 角色模型由业务注入(options.Setting.GetRole,players.Start 有前置校验),
// Updater 已销毁(released)或业务返回 nil 时回空串——本函数会被非业务协程调用,不得 panic
func (p *Player) Guid() string {
	if p.Updater == nil {
		return ""
	}
	if role := options.Setting.GetRole(p.Updater); role != nil {
		return role.GetGuid()
	}
	return ""
}

// Unload 卸载玩家:未落库的改动刷进数据库,成功后把 Updater 与组件从内存拆掉(库里的数据不动)。
// 调用者必须持有玩家锁(p.Lock),否则会与在途业务调用竞态置空 Updater。
//
// 🔴 **仅供 players daemon 回收调用**:要求状态已是 StatusReleased(daemon 在锁外 CAS 置好再调),
// 否则直接返回错误、什么都不做。业务层在正常状态下误调用的话,Updater 被抽走——本次 handler 后续
// 代码与框架收尾的 Submit 当场 nil panic,对象还留在 Manage 里成了在线的空壳——所以宁可报错。
// 要让内存跟上带外改库用 p.Updater.Reload();要整个退役用 players.Terminate。
func (p *Player) Unload() error {
	if s := atomic.LoadInt32(&p.Status); s != StatusReleased {
		return fmt.Errorf("player unload denied: status %v is not released(uid:%s)", s, p.uid)
	}
	//Updater==nil(没有数据要卸载,如新对象建 Updater 失败)直接算成功。
	//不能返回 error:daemon 的 released 收到 error 会还原状态、下个 tick 重试,
	//每次都失败就成了死循环。
	if p.Updater != nil {
		if err := p.Updater.Unload(); err != nil {
			return err
		}
	}
	p.Updater = nil
	p.Pending = Pending{}
	p.Emitter = nil
	p.Inbox = nil
	//组件与 Updater 一起丢:createComponents 以 Times 判「已建」,不清的话对象万一被复用重新 Loading,
	//Condition / Emitter / Inbox 不会重建,之后一 Emit 就 nil panic
	p.Times = nil
	p.Condition = nil
	return nil
}

// On 注册玩家事件监听(提交期派发)。judge 为裁决参数(如 condition.Array),可为 nil 表示不裁决
func (p *Player) On(t int32, judge condition.Judge, handle emitter.Callback) (r *emitter.Context) {
	return p.Emitter.On(t, judge, handle)
}

func (p *Player) Emit(t int32, v int32, args ...int32) {
	p.Emitter.Emit(t, v, args...)
}

// Listen 使用name注册监听避免重复,同名覆盖参数和回调(提交期派发)
func (p *Player) Listen(name string, t int32, judge condition.Judge, handle emitter.Listener) (r *emitter.Context, err error) {
	return p.Emitter.Listen(name, t, judge, handle)
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
func (p *Player) AddItems(items any, multi ...int64) {
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
func (p *Player) SubItems(items any, multi ...int64) {
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
