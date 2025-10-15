package context

import (
	"encoding/json"
	"errors"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/hwcer/cosgo/utils"
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
	//开发者模式,GM指令
	if this.Guid != "" && options.Options.Developer {
		if err = this.validateAccountComprehensive(this.Guid); err != nil {
			return
		}
		r.Guid = this.Guid
		r.Developer = true
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

// 综合验证函数
func (this *Authorize) validateAccountComprehensive(account string) error {
	// 检查长度
	if utf8.RuneCountInString(account) < 2 || utf8.RuneCountInString(account) > 20 {
		return errors.New("账号长度必须在2-20个字符之间")
	}

	// 检查是否包含不可见字符
	for _, char := range account {
		if !unicode.IsGraphic(char) || unicode.IsControl(char) {
			return errors.New("账号不能包含不可见字符或控制字符")
		}
	}

	// 检查是否只包含允许的字符（可选）
	pattern := `^[a-zA-Z0-9_\-\@\.]+$`
	matched, _ := regexp.MatchString(pattern, account)
	if !matched {
		return errors.New("账号只能包含字母 数字 @ . - _字符")
	}

	return nil
}
