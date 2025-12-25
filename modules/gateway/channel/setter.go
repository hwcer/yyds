package channel

import "github.com/hwcer/cosgo/session"

const PlayerChannelName = "PlayerChannel"

type PlayerChannelValue map[string]string

func (p PlayerChannelValue) Clone() PlayerChannelValue {
	r := make(PlayerChannelValue, len(p))
	for k, v := range p {
		r[k] = v
	}
	return r
}

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
			v, _ := i.(PlayerChannelValue)
			cv = v.Clone()
		} else {
			cv = make(PlayerChannelValue)
		}
		old, ok = cv[name]
		cv[name] = value
		setter.Set(PlayerChannelName, cv)
	})
	return
}

func (s *Setter) Leave(name string, value string) (ok bool) {
	var old string
	s.Mutex(func(setter session.Setter) {
		var cv PlayerChannelValue
		if i := setter.Get(PlayerChannelName); i != nil {
			v, _ := i.(PlayerChannelValue)
			cv = v.Clone()
		} else {
			cv = make(PlayerChannelValue)
		}
		if old, ok = cv[name]; ok && old == value {
			delete(cv, name)
		}
		setter.Set(PlayerChannelName, cv)
	})
	return
}

// Get 获取 是否在渠道中 以及渠道值
func (s *Setter) Get(name string) (value string, ok bool) {
	i := s.Data.Get(PlayerChannelName)
	if i == nil {
		return "", false
	}
	v, ok := i.(PlayerChannelValue)
	if !ok {
		return "", false
	}
	value, ok = v[name]

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
