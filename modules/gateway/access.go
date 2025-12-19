package gateway

import (
	"fmt"

	"github.com/hwcer/cosgo/session"
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
