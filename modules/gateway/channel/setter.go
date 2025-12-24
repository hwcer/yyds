package channel

import "github.com/hwcer/cosgo/session"

const PlayerChannelName = "PlayerChannel"

type PlayerChannelValue map[string]string

type Setter struct {
	*session.Data
}

func NewSetter(d *session.Data) *Setter {
	return &Setter{d}
}

func (s *Setter) Join(name, value string) (old string, ok bool) {
	s.Mutex(func(setter session.Setter) {
		var cv PlayerChannelValue
		if i := setter.Get(PlayerChannelName); i != nil {
			cv = i.(PlayerChannelValue)
		} else {
			cv = make(PlayerChannelValue)
			setter.Set(PlayerChannelName, cv)
		}
		old, ok = cv[name]
		cv[name] = value
	})
	return
}

func (s *Setter) Leave(name string, value string) (ok bool) {
	var old string
	s.Mutex(func(setter session.Setter) {
		var cv PlayerChannelValue
		if i := setter.Get(PlayerChannelName); i != nil {
			cv = i.(PlayerChannelValue)
			if old, ok = cv[name]; ok && old == value {
				delete(cv, name)
			}
		}
	})
	return
}

type kv struct {
	k string
	v string
}

func (s *Setter) Release() (rs []kv) {
	var cv PlayerChannelValue
	s.Mutex(func(setter session.Setter) {
		if i := setter.Get(PlayerChannelName); i != nil {
			cv = i.(PlayerChannelValue)
			for k, v := range cv {
				rs = append(rs, kv{k, v})
			}
		}
		setter.Delete(PlayerChannelName)
	})
	return
}
