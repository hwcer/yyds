package gateway

import (
	"github.com/hwcer/cosnet"
	"github.com/hwcer/cosrpc/xshare"
	"github.com/hwcer/coswss"
	"github.com/hwcer/yyds/gateway/players"
	"github.com/hwcer/yyds/options"
	"net/http"
)

func init() {
	coswss.Options.Verify = WSVerify
	coswss.Options.Accept = WSAccept
}

func WSVerify(w http.ResponseWriter, r *http.Request) (meta map[string]string, err error) {
	//logger.Trace("Sec-Websocket-Extensions:%v", r.Head.Get("Sec-Websocket-Extensions"))
	//logger.Trace("Sec-Websocket-Key:%v", r.Head.Get("Sec-Websocket-Key"))
	//logger.Trace("Sec-Websocket-Protocol:%v", r.Head.Get("Sec-Websocket-Protocol"))
	//logger.Trace("Sec-Websocket-Branch:%v", r.Head.Get("Sec-Websocket-Branch"))
	token := r.Header.Get("Sec-Websocket-Protocol")
	if token == "" || len(token) < 2 {
		//return nil, values.Error("token empty")
		return nil, nil
	}
	req := xshare.NewMetadata()
	res := xshare.NewMetadata()
	reply := make([]byte, 0)

	err = request(nil, options.Gate.Login, []byte{}, req, res, &reply)
	if err != nil {
		return nil, err
	}

	//sess := session.New()
	//if err = sess.Verify(token); err != nil {
	//	return "", values.Parse(err)
	//}
	//uuid = res[options.ServiceMetadataGUID]
	return res, nil
}
func WSAccept(s *cosnet.Socket, meta map[string]string) {
	if len(meta) == 0 {
		return
	}
	uuid, ok := meta[options.ServiceMetadataGUID]
	if !ok {
		return
	}
	_, _ = players.Players.Binding(s, uuid, CookiesFilter(meta))
	return
}
