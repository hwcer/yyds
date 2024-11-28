package share

var Configs = struct {
	Task    func(id int32) TaskConfig
	Ticket  func(id int32) TicketConfig
	Emitter func(id int32) EmitterConfig
}{}

type TaskConfig interface {
	GetKey() int32
	GetArgs() []int32
	GetGoal() int32
	GetCondition() int32
}

type EmitterConfig interface {
	GetDaily() int32
	GetRecord() int32
	GetEvents() int32
	GetReplace() int32
	GetUpdate() int32
}

type TicketConfig interface {
	GetDot() []int32
	GetLimit() []int32
	GetCycle() []int32
}
