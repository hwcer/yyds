package share

import (
	"errors"
	"github.com/hwcer/cosgo/logger"
	"github.com/hwcer/cosgo/utils"
	"strconv"
	"strings"
)

var Host = struct {
	Encode func(address string, room string) string
	Decode func(token string) (address string, room string, err error)
}{
	Encode: func(address string, room string) string {
		s := utils.Ipv4Encode(address)
		var arr []string
		arr = append(arr, strconv.FormatUint(s, 32))
		arr = append(arr, room)
		return strings.Join(arr, "x")
	},
	Decode: func(token string) (address string, room string, err error) {
		arr := strings.SplitN(token, "x", 2)
		if len(arr) != 2 {
			err = errors.New("token error")
			logger.Debug("token error, %s", token)
			return
		}
		room = arr[1]
		var ips uint64
		ips, err = strconv.ParseUint(arr[0], 32, 64)
		if err != nil {
			return
		}
		address = utils.Ipv4Decode(ips)
		return
	},
}
