package updater

type EventType int8

const (
	OnLoaded EventType = iota //加载之后执行
	OnPreData
	OnPreVerify  //verify前
	OnPreSubmit  //submit 前
	OnPreRelease //Release 释放前
)

// Listener 监听任务,返回true表示继续监听,false 从监听列表中移除
type Listener func(u *Updater) (next bool)

// Middleware 监听中间件，所有EventType都会调用 Emit 直到返回false从列表中移除
type Middleware interface {
	Emit(u *Updater, t EventType) (next bool)
}

type Events struct {
	events map[EventType][]Listener //过程事件
	//emitter     map[EventType][]Listener //常驻事件
	middlewares map[string]Middleware //中间件
}

// On 监听事件,必须在事件回调返回false时删除事件
func (e *Events) On(t EventType, handle Listener) {
	if e.events == nil {
		e.events = map[EventType][]Listener{}
	}
	e.events[t] = append(e.events[t], handle)
}

// Get 获取中间件
func (e *Events) Get(name string) any {
	if e.middlewares != nil && e.middlewares[name] != nil {
		return e.middlewares[name]
	}
	return nil
}
func (e *Events) Use(name string, handle Middleware) bool {
	return e.Set(name, handle)
}

// Set 设置中间件,如果已存在返回false
func (e *Events) Set(name string, handle Middleware) bool {
	if e.middlewares == nil {
		e.middlewares = map[string]Middleware{}
	}
	if _, ok := e.middlewares[name]; ok {
		return false
	}
	e.middlewares[name] = handle
	return true
}

func (e *Events) LoadOrStore(name string, handle Middleware) (v Middleware) {
	if e.middlewares == nil {
		e.middlewares = map[string]Middleware{}
	}
	if v = e.middlewares[name]; v == nil {
		v = handle
		e.middlewares[name] = handle
	}
	return
}

func (e *Events) LoadOrCreate(name string, creator func() Middleware) (v Middleware) {
	if e.middlewares == nil {
		e.middlewares = map[string]Middleware{}
	}
	if v = e.middlewares[name]; v == nil {
		v = creator()
		e.middlewares[name] = v
	}
	return
}

func (e *Events) emit(u *Updater, t EventType) {
	if u.Error != nil {
		return
	}
	e.triggerGlobal(u, t)
	e.triggerEvents(u, t)
	e.triggerMiddleware(u, t)
}

func (e *Events) release() {
	//e.events = nil
	//e.middlewares = nil
}

func (e *Events) triggerGlobal(u *Updater, t EventType) {
	if l := globalEvents[t]; len(l) > 0 {
		for _, h := range l {
			h(u)
		}
	}
	return
}
func (e *Events) triggerEvents(u *Updater, t EventType) {
	if events := e.events[t]; len(events) > 0 {
		var es []Listener
		for _, h := range events {
			if h(u) {
				es = append(es, h)
			}
		}
		e.events[t] = es
	}
	return
}

func (e *Events) triggerMiddleware(u *Updater, t EventType) {
	if len(e.middlewares) == 0 {
		return
	}
	mw := map[string]Middleware{}
	for k, p := range e.middlewares {
		if p.Emit(u, t) {
			mw[k] = p
		}
	}
	e.middlewares = mw
}
