package gateway

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
)

type Token struct {
	Guid      string `json:"openid"`
	Appid     string `json:"appid"`
	Expire    int64  `json:"expire"`
	Developer bool   `json:"developer"`
}

type Authorize struct {
	Guid   string `json:"guid"`
	Access string `json:"access"`
	Secret string `json:"secret"`
}

func (this *Authorize) Verify() (r *Token, err error) {
	r = &Token{}
	//是否开启GM
	if this.Secret != "" {
		if options.Options.Developer == "" {
			return nil, errors.New("GM commands are disabled")
		}
		if this.Secret != options.Options.Developer {
			return nil, errors.New("GM commands error")
		}
		r.Developer = true
	}
	//GM模式允许快速登录
	if this.Guid != "" && r.Developer {
		if err = this.validateAccountComprehensive(this.Guid); err != nil {
			return
		}
		r.Guid = this.Guid
		return
	}
	//正常游戏模式
	if this.Access == "" {
		return nil, session.ErrorSessionEmpty
	}
	if options.Options.Secret == "" {
		return nil, session.Errorf("Options.Secret is empty")
	}
	var s string
	if s, err = utils.Crypto.GCMDecrypt(this.Access, options.Options.Secret, nil); err != nil {
		return nil, session.Errorf(err)
	}
	if err = json.Unmarshal([]byte(s), r); err != nil {
		return nil, session.Errorf(err)
	}
	if r.Guid == "" {
		return nil, session.Errorf("access guid empty")
	}
	if r.Expire > 0 && r.Expire < time.Now().Unix() {
		return nil, session.ErrorSessionExpired
	}
	if r.Appid != options.Options.Appid {
		return nil, session.Errorf("access appid error")
	}
	if options.Options.Maintenance && !r.Developer {
		return nil, errors.ErrServerMaintenance
	}
	return
}

// 综合验证函数
func (this *Authorize) validateAccountComprehensive(account string) error {
	// 检查是否只包含允许的字符（可选）
	pattern := `^[a-zA-Z0-9~!@#$%^&*()_+\-=\[\]\\{}|;':",./<>?]{2,64}$`
	matched, _ := regexp.MatchString(pattern, account)
	if !matched {
		return fmt.Errorf("账号规则 %s", pattern)
	}

	return nil
}
