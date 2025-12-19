package gateway

import (
	"net/http"

	"github.com/hwcer/cosnet"
	"github.com/hwcer/coswss"
	"github.com/hwcer/logger"
	"github.com/hwcer/yyds/modules/gateway/players"
	"github.com/hwcer/yyds/options"
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
	//token := r.Header.Get("Sec-Websocket-Protocol")
	//if token == "" || len(token) < 2 {
	//	//return nil, values.Error("token empty")
	//	return nil, nil
	//}
	//req := values.Metadata{}
	//res := values.Metadata{}
	//reply := make([]byte, 0)

	//err = request(nil, options.Gate.Login, []byte{}, req, res, &reply)
	//if err != nil {
	//	return nil, err
	//}

	//sess := session.New()
	//if err = sess.G2SOAuth(token); err != nil {
	//	return "", values.Parse(err)
	//}
	//uuid = res[options.ServiceMetadataGUID]
	return nil, nil
}
func WSAccept(sock *cosnet.Socket, meta map[string]string) {
	if len(meta) == 0 {
		return
	}
	uuid, ok := meta[options.ServiceMetadataGUID]
	if !ok {
		return
	}
	value := CookiesFilter(meta)
	if _, err := players.Connect(sock, uuid, value); err != nil {
		logger.Alert("wss session create fail:%v", err)
	}

}
