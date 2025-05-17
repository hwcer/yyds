package gateway

import (
	"time"
)

const elapsedMillisecond = 200 * time.Millisecond

// Formatter 格式化路径
//var Formatter = func(s string) string {
//	return strings.ToLower(s)
//}

//var Writer = func(c *cosweb.Context, reply []byte, cookie *http.Cookie) error {
//	return c.Bytes(cosweb.ContentTypeApplicationJSON, reply)
//}

/*
	if cookie != nil {
		r := map[string]any{}
		if err = json.Unmarshal(reply, &r); err != nil {
			return c.JSON(values.Parse(err))
		}

		//r["cookie"] = map[string]string{"Name": cookie.Name, "Value": cookie.Value}
		return c.JSON(r)
	} else {
		return c.Bytes(cosweb.ContentTypeApplicationJSON, reply)
	}
*/
