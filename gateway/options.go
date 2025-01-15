package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosrpc/xshare"
)

var Options = struct {
	Query func(player *session.Data, meta xshare.Metadata) map[string]any //Query 生成参数列表
	Proxy func(player *session.Data, meta xshare.Metadata)                //定制转发metadata
}{
	//Query: func(player *session.Data, meta xshare.Metadata) values.Values { return make(values.Values) },
}
