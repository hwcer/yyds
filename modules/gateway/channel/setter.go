package channel

import (
	"strings"

	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/logger"
)

const PlayerChannelPrefix = "_c_p."

type Setter struct {
	*session.Data
}

func NewSetter(d *session.Data) *Setter {
	return &Setter{d}
}

func (s *Setter) Name(name string) string {
	return strings.Join([]string{PlayerChannelPrefix, name}, "")
}
func (s *Setter) Trim(name string) string {
	return strings.TrimPrefix(name, PlayerChannelPrefix)
}

func (s *Setter) get(rk string) (v string, ok bool) {
	var i any
	if i = s.Data.Get(rk); i != nil {
		var valueOk bool
		if v, valueOk = i.(string); valueOk {
			return v, true
		}
		// 类型断言失败，记录错误日志
		logger.Error("setter.get type assertion failed for key:%s value:%v", rk, i)
	}
	return "", false
}

func (s *Setter) Join(name, value string) (old string, ok bool) {
	rk := s.Name(name)
	old, ok = s.get(rk)
	s.Data.Set(rk, value)
	return
}

func (s *Setter) Leave(name string) {
	rk := s.Name(name)
	s.Data.Delete(rk)
}

// Get 获取是否在频道中以及频道值
func (s *Setter) Get(name string) (value string, ok bool) {
	rk := s.Name(name)
	return s.get(rk)
}

type kv struct {
	k string
	v string
}

func (s *Setter) Release() (rs []kv) {
	var rk []string
	s.Data.Range(func(k string, v any) bool {
		if strings.HasPrefix(k, PlayerChannelPrefix) {
			rk = append(rk, k)
			if vStr, ok := v.(string); ok {
				rs = append(rs, kv{k: s.Trim(k), v: vStr})
			} else {
				// 类型断言失败，记录错误日志
				logger.Error("setter.Release type assertion failed for key:%s value:%v", k, v)
			}
		}
		return true
	})
	if len(rs) == 0 {
		return
	}
	s.Data.Mutex(func(setter session.Setter) {
		for _, k := range rk {
			setter.Delete(k)
		}
	})

	return
}
