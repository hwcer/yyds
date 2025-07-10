package rooms

import (
	"sync"
)

var Players = players{Map: sync.Map{}}
var PMSMutex sync.Mutex

type PMS struct {
	dict map[string]*Room // room id-> Room
}

func NewPMS() *PMS {
	return &PMS{dict: make(map[string]*Room)}
}

type players struct {
	sync.Map
}

func (this *players) Get(uuid string) *PMS {
	if i, ok := this.Map.Load(uuid); ok {
		return i.(*PMS)
	}
	return nil

}

func (this *players) Load(uuid string) *PMS {
	i, _ := this.Map.LoadOrStore(uuid, NewPMS)
	return i.(*PMS)
}

func (this *players) Delete(uuid string) *PMS {
	i, loader := this.Map.LoadAndDelete(uuid)
	if loader {
		return i.(*PMS)
	}
	return nil
}

func (this *PMS) Has(name string) bool {
	_, ok := this.dict[name]
	return ok
}

func (this *PMS) Get(name string) *Room {
	return this.dict[name]
}

func (this *PMS) Set(name string, room *Room) {
	PMSMutex.Lock()
	defer PMSMutex.Unlock()
	dict := make(map[string]*Room)
	for k, v := range this.dict {
		dict[k] = v
	}
	dict[name] = room
	this.dict = dict
}

func (this *PMS) Delete(names ...string) {
	PMSMutex.Lock()
	defer PMSMutex.Unlock()
	dict := make(map[string]*Room)
	for k, v := range this.dict {
		dict[k] = v
	}
	for _, name := range names {
		delete(this.dict, name)
	}
	this.dict = dict
}

func (this *PMS) remove(name string) {
	dict := make(map[string]*Room)
	for k, v := range this.dict {
		if k != name {
			dict[k] = v
		}
	}
	this.dict = dict
}
