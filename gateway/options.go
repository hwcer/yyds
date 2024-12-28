package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
)

var Options = struct {
	Query func(player *session.Data) values.Values //Query 生成参数列表
}{
	Query: func(player *session.Data) values.Values { return make(values.Values) },
}
