package itype

import (
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/times"
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/operator"
	"github.com/hwcer/yyds/kernel/config"
	"github.com/hwcer/yyds/kernel/model"
	"github.com/hwcer/yyds/kernel/share"
)

const (
	ticketPlugName = "_model_ticket_plug"
)

var Ticket = &TicketIType{}

type cycleHandle func(dateTime *times.Times, powerTime int64, powerMax int64, cycle int64) (addVal int64, newTime int64)

var cycleHandleDict = make(map[int32]cycleHandle)

func init() {
	Ticket.itemsIType = NewItemsIType(config.ITypeTicket)
	cycleHandleDict[1] = cycleHandleType1
	cycleHandleDict[2] = cycleHandleType2
}

type TicketIType struct {
	*itemsIType
}

func (this *TicketIType) Listener(u *updater.Updater, op *operator.Operator) {
	if share.Configs.Ticket == nil {
		logger.Alert("ITypes.Ticket GetConfig is nil")
		return
	}
	this.Settlement(u, op.IID)
}

// Settlement 强制结算体力
func (this *TicketIType) Settlement(u *updater.Updater, iid ...int32) {
	plug := u.Events.LoadOrCreate(ticketPlugName, this.createTicketPlug).(*ticketPlug)
	for _, id := range iid {
		c := share.Configs.Ticket(id)
		if c == nil {
			continue
		}
		limit := c.GetLimit()
		if limit[0] > 0 && limit[1] > 0 {
			u.Select(limit[0])
		}
		plug.add(id)
	}
}

func (this *TicketIType) createTicketPlug() updater.Middleware {
	return &ticketPlug{}
}

type ticketPlug struct {
	dict map[int32]bool
}

func (this *ticketPlug) Emit(u *updater.Updater, t updater.EventType) bool {
	if t == updater.OnPreVerify {
		return this.checkAllTicket(u)
	}
	return true
}
func (this *ticketPlug) add(iid int32) {
	if this.dict == nil {
		this.dict = map[int32]bool{}
	}
	this.dict[iid] = true
}

func (this *ticketPlug) checkAllTicket(u *updater.Updater) bool {
	for iid, _ := range this.dict {
		if v := u.Get(iid); v != nil {
			this.sumTicket(u, v.(*model.Items).Copy())
		} else {
			this.newTicket(u, iid)
		}
	}
	return false
}

func (this *ticketPlug) powerMax(u *updater.Updater, iid int32) int64 {
	c := share.Configs.Ticket(iid)
	limit := c.GetLimit()
	powerMax := int64(limit[2])
	if limit[0] > 0 && limit[1] > 0 {
		powerMax += u.Val(limit[0]) * int64(limit[1]) / 10000
	}
	return powerMax
}

func (this *ticketPlug) newTicket(u *updater.Updater, iid int32) {
	i, err := Ticket.Create(u, iid, this.powerMax(u, iid))
	if err != nil {
		logger.Debug("Ticket ObjectId error:%v", err)
		return
	}
	op := &operator.Operator{}
	op.OID = i.OID
	op.IID = i.IID
	op.Type = operator.TypesNew
	op.Value = i.Value
	op.Result = []any{i}
	_ = u.Operator(op, true)
}

func (this *ticketPlug) sumTicket(u *updater.Updater, data *model.Items) {
	c := share.Configs.Ticket(data.IID)
	t := times.New(u.Time)
	nowTime := t.Now().Unix()
	powerMax := this.powerMax(u, data.IID)
	powerTime := data.Attach.GetInt64(ItemsAttachTypeTicket)

	var value int64
	var attach int64

	if data.Value >= powerMax {
		attach = nowTime
	} else if powerTime == 0 {
		//初始回满
		attach = nowTime
		if data.Value < powerMax {
			value = powerMax
		}
	} else {
		var addVal int64
		var newTime int64
		//每日，周回复
		cycle := c.GetCycle()
		if f := cycleHandleDict[cycle[0]]; f != nil {
			addVal, newTime = f(t, powerTime, powerMax, int64(cycle[1]))
		}
		//计时回复
		dot := c.GetDot()
		if powerTime < nowTime && dot[0] > 0 && dot[1] > 0 {
			dotNum := int64(dot[0])
			diffTime := nowTime - powerTime
			retNum := diffTime / dotNum * int64(dot[1])
			if retNum > 0 {
				lastTime := powerTime + retNum*dotNum
				if lastTime > newTime {
					newTime = lastTime
				}
				addVal += retNum
			}
		}
		if newTime > 0 {
			attach = newTime
		}

		if value = data.Value + addVal; value > powerMax {
			value = powerMax
		}

	}

	if value != data.Value || attach != powerTime {
		v := dataset.Update{}
		if value > 0 {
			v["val"] = value
		}
		if attach > 0 {
			v["att"] = attach
		}

		op := &operator.Operator{}
		op.OID = data.OID
		op.IID = data.IID
		op.Type = operator.TypesSet
		op.Result = v
		if err := u.Operator(op, true); err != nil {
			logger.Alert(err)
		}
	}
}

// 每日回复
func cycleHandleType1(t *times.Times, powerTime int64, powerMax, cycle int64) (addVal int64, newTime int64) {
	lastTime := t.Daily(0).Unix()
	if powerTime >= lastTime {
		return
	}
	newTime = lastTime
	if cycle > 0 {
		addVal = cycle
	} else {
		addVal = powerMax
	}
	return
}

// 每周回复
func cycleHandleType2(t *times.Times, powerTime int64, powerMax, cycle int64) (addVal int64, newTime int64) {
	lastTime := t.Weekly(0).Unix()
	if powerTime >= lastTime {
		return
	}
	newTime = lastTime
	if cycle > 0 {
		addVal = cycle
	} else {
		addVal = powerMax
	}
	return
}
