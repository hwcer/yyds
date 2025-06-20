package context

import (
	"encoding/json"
	"errors"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/yyds/options"
	"time"
)

type Token struct {
	Guid   string `json:"openid"`
	Appid  string `json:"appid"`
	Expire int64  `json:"expire"`
}

type Authorize struct {
	Guid   string `json:"guid"`
	Access string `json:"access"`
	Secret string `json:"secret"`
}

func (this *Authorize) Verify() (r *Token, err error) {
	r = &Token{}
	//开发者模式,GM指令
	if this.Guid != "" && (options.Game.Developer || (this.Secret != "" && this.Secret == options.Game.Secret)) {
		r.Guid = this.Guid
		return
	}
	//正常游戏模式
	if this.Access == "" {
		return nil, errors.New("access empty")
	}
	if options.Options.Secret == "" {
		return nil, errors.New("未开启平台授权登录方式")
	}
	var s string
	if s, err = utils.Crypto.GCMDecrypt(this.Access, options.Options.Secret, nil); err != nil {
		return
	}
	if err = json.Unmarshal([]byte(s), r); err != nil {
		return
	}
	if r.Guid == "" {
		return nil, errors.New("openid empty")
	}
	if r.Expire > 0 && r.Expire < time.Now().Unix() {
		return nil, errors.New("access expire")
	}
	if r.Appid != options.Options.Appid {
		return nil, errors.New("access error")
	}
	return
}
