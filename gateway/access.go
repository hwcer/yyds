package gateway

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/utils"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosnet"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
)

// 接口权限判定 必须注册所有 options.OAuthType

var Access = access{}

func init() {
	Access.Register(options.OAuthTypeNone, Access.OAuthTypeNone)
	Access.Register(options.OAuthTypeOAuth, Access.OAuthTypeOAuth)
	Access.Register(options.OAuthTypeSelect, Access.OAuthTypeSelect)
	Access.Register(options.OAuthTypePlayer, Access.OAuthTypeSelect)
}

type accessSocket interface {
	Socket() *cosnet.Socket
}

type accessFunc func(r Request, req values.Metadata, isMaster bool) (*session.Data, error)

type access struct {
	dict map[options.OAuthType]accessFunc
}

func (this *access) Register(l options.OAuthType, f accessFunc) {
	if this.dict == nil {
		this.dict = make(map[options.OAuthType]accessFunc)
	}
	this.dict[l] = f
}

func (this *access) oauth(r Request, req values.Metadata) (p *session.Data, err error) {
	if p, err = r.Cookie(); err != nil {
		return nil, err
	} else if p == nil {
		return nil, errors.ErrLogin
	}
	return
}

// OAuthTypeNone 普通接口
func (this *access) OAuthTypeNone(r Request, req values.Metadata, isMaster bool) (p *session.Data, err error) {
	if f, ok := r.(accessSocket); ok {
		sock := f.Socket()
		req[options.ServiceSocketId] = fmt.Sprintf("%d", sock.Id())
	}
	req[options.ServiceClientIp] = r.RemoteAddr()
	return
}

// OAuthTypeOAuth 账号登录
func (this *access) OAuthTypeOAuth(r Request, req values.Metadata, needMaster bool) (p *session.Data, err error) {
	if p, err = this.oauth(r, req); err != nil {
		return nil, err
	}
	if uuid := p.UUID(); uuid == "" {
		return nil, errors.ErrLogin
	} else {
		req[options.ServiceMetadataGUID] = uuid
	}
	req[options.ServiceClientIp] = r.RemoteAddr()
	if needMaster && !this.HasMaster(p) {
		err = errors.ErrNeedGameMaster
	}
	return
}

// OAuthTypeSelect 必须选择角色
func (this *access) OAuthTypeSelect(r Request, req values.Metadata, needMaster bool) (p *session.Data, err error) {
	if p, err = this.oauth(r, req); err != nil {
		return nil, err
	}
	if uid := p.GetString(options.ServiceMetadataUID); uid == "" {
		return nil, errors.ErrNotSelectRole
	} else {
		req[options.ServiceMetadataUID] = p.GetString(options.ServiceMetadataUID)
	}
	if needMaster && !this.HasMaster(p) {
		err = errors.ErrNeedGameMaster
	}
	return
}

// HasMaster 是GM
func (this *access) HasMaster(p *session.Data) bool {
	if p == nil {
		return false
	}
	if gm := p.GetInt32(options.ServiceMetadataDeveloper); gm == 1 {
		return true
	}
	return false
}

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
	//开发者模式,GM指令
	if this.Guid != "" && r.Developer {
		//if this.Guid != "" {
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
