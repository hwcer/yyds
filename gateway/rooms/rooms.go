package rooms

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
	"strings"
	"sync"
)

const (
	SessionPlayerRoomsName = "player.rooms"
)

var rooms = sync.Map{}

func Get(name string) (r *Room) {
	if i, ok := rooms.Load(name); ok {
		r = i.(*Room)
	}
	return
}

// All 所有房间
func All(p *session.Data) (r map[string]struct{}) {
	if i := p.Get(SessionPlayerRoomsName); i != nil {
		for k, v := range i.(map[string]struct{}) {
			r[k] = v
		}
	}
	return
}

func loadOrCreate(name string) (r *Room, loaded bool) {
	i, loaded := rooms.LoadOrStore(name, &Room{})
	r = i.(*Room)
	return
}

func Join(name string, p *session.Data) {
	prs := All(p)
	var changed bool
	for _, k := range strings.Split(name, ",") {
		if room, _ := loadOrCreate(k); room != nil {
			if room.Join(p) {
				changed = true
				prs[k] = struct{}{}
			}
		}
	}
	if changed {
		p.Set(SessionPlayerRoomsName, prs)
	}
}

func Leave(name string, p *session.Data) {
	prs := All(p)
	var changed bool
	for _, k := range strings.Split(name, ",") {
		if room := Get(k); room != nil {
			if room.Leave(p) {
				changed = true
				delete(prs, k)
			}
		}
	}
	if changed {
		p.Set(SessionPlayerRoomsName, prs)
	}
}

func Release(p *session.Data) {
	prs := All(p)
	for k, _ := range prs {
		if room := Get(k); room != nil {
			room.Leave(p)
		}
	}
	p.Set(SessionPlayerRoomsName, map[string]struct{}{})
}

func Range(name string, f func(*session.Data) bool) {
	room := Get(name)
	if room == nil {
		return
	}
	room.Range(f)
}

func Broadcast(c *xshare.Context) any {
	path := c.GetMetadata(options.ServiceMessagePath)
	name := c.GetMetadata(options.ServiceMessageRoom)
	room := Get(name)
	if room == nil {
		return false
	}

	ignore := c.GetMetadata(options.ServiceMessageIgnore)
	ignoreMap := make(map[string]struct{})
	if ignore != "" {
		arr := strings.Split(ignore, ",")
		for _, v := range arr {
			ignoreMap[v] = struct{}{}
		}
	}
	body := c.Bytes()

	for _, p := range room.ps {
		uid := p.GetString(options.ServiceMetadataUID)
		if _, ok := ignoreMap[uid]; ok {
			socket := players.Socket(p)
			if socket != nil {
				_ = socket.Send(path, nil, body)
			}
		}
	}
	return true
}