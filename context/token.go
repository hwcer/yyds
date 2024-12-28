package context

import (
	"encoding/json"
	"errors"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/yyds/options"
	"strings"
	"time"
)

type Token interface {
	GetGuid() string
	GetAppid() string
}

type defaultToken struct {
	Guid   string
	Appid  string
	Expire int64
}

func (this *defaultToken) GetGuid() string {
	return this.Guid
}
func (this *defaultToken) GetAppid() string {
	return this.Appid
}

func Verify(c *Context) (r Token, err error) {

	d := &defaultToken{}
	r = d

	//开发者模式
	if options.Game.Developer {
		if guid := c.GetString("guid"); guid != "" {
			d.Guid = guid
			return
		} else if username := strings.TrimSpace(c.GetString("username")); username != "" {
			d.Guid = username
			return
		}
	}
	//正常游戏模式
	access := c.GetString("access")
	if access == "" {
		return nil, errors.New("[release model]access empty")
	}
	if options.Options.Secret == "" {
		return nil, errors.New("未开启平台授权登录方式")
	}
	var s string
	if s, err = utils.Crypto.GCMDecrypt(access, options.Options.Secret, nil); err != nil {
		return
	}
	if err = json.Unmarshal([]byte(s), d); err != nil {
		return
	}
	if d.Expire > 0 && d.Expire < time.Now().Unix() {
		return nil, errors.New("access expire")
	}
	if d.Appid != options.Options.Appid {
		return nil, errors.New("access error")
	}
	return
}
