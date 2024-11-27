package player

import (
	"github.com/hwcer/updater"
	"github.com/hwcer/updater/dataset"
	"github.com/hwcer/updater/emitter"
	"github.com/hwcer/updater/verify"
	"server/config"
	"server/define"
	"server/game/model"
)

const taskListenerKey = "task_listener_key"

func init() {
	Register("TaskTargetWorker", taskTargetWorker)
}

func taskTargetWorker(p *Player) any {
	if !p.Task.Collection.Loader() {
		return nil
	}
	p.Task.Range(func(id string, data *model.Task) bool {
		if data.Status == model.TaskStatusStart {
			p.Task.listener(data.IID)
		}
		return true
	})
	return nil
}

type Task struct {
	*updater.Collection
	player    *Player
	listeners map[int32]struct{} //已经存在的监听
}

type TaskTarget struct {
	k int32
}

func (tt *TaskTarget) GetKey() int32 {
	return tt.k
}
func (tt *TaskTarget) GetArgs() []int32 {
	return nil
}
func (tt *TaskTarget) GetGoal() int32 {
	return 1
}
func (tt *TaskTarget) GetCondition() int32 {
	return verify.ConditionData
}

func NewTask(p *Player) *Task {
	doc := p.Collection(define.ITypeTask)
	r := &Task{Collection: doc, player: p}
	r.listeners = map[int32]struct{}{}
	return r
}

// Auto  自动检查是否已经完成[ids]中的所有任务
func (this *Task) Auto(ids ...int32) {
	for _, id := range ids {
		if id > 0 {
			this.player.Verify.Auto(&TaskTarget{k: id})
		}
	}
}

// Get 总是返回TASK对象
func (this *Task) Get(id int32) (r *model.Task) {
	if v := this.Collection.Get(id); v == nil {
		return
	} else {
		r = v.(*model.Task)
	}
	return
}

func (this *Task) listener(id int32) {
	if _, ok := this.listeners[id]; ok {
		return
	}
	c := config.Data.Task[id]
	if c == nil || c.GetCondition() != verify.ConditionEvents {
		return
	}

	this.listeners[id] = struct{}{}
	l := this.player.Emitter.Listener(c.GetKey(), c.GetArgs(), this.handle)
	l.Attach.Set(taskListenerKey, id)
}

func (this *Task) handle(l *emitter.Listener, val int32) (r bool) {
	id := l.Attach.GetInt32(taskListenerKey)
	c := config.Data.Task[id]
	if c == nil {
		return false
	}
	d := this.Get(id)
	if d == nil {
		return false
	}
	if r = d.Target+int64(val) < int64(c.GetGoal()); !r {
		delete(this.listeners, id)
	}
	this.Add(id, val)
	return
}

// Update 更新任务状态,如果不存在则插入,需要先select(iid),然后Date
func (this *Task) Update(iid int32, update dataset.Update) error {
	if _, ok := update["update"]; !ok {
		update["update"] = this.Updater.Time.Unix()
	}
	this.Set(iid, update)
	this.listener(iid)
	return nil
}

func (this *Task) Range(h func(id string, active *model.Task) bool) {
	this.Collection.Range(func(id string, doc *dataset.Document) bool {
		v, _ := doc.Any().(*model.Task)
		return h(id, v)
	})
}
