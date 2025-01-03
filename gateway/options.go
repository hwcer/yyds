package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosrpc/xshare"
)

var Options = struct {
	Query func(player *session.Data, meta xshare.Metadata) values.Values //Query 生成参数列表
}{
	Query: func(player *session.Data, meta xshare.Metadata) values.Values { return make(values.Values) },
}
