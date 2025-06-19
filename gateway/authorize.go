package gateway

import (
	"github.com/hwcer/cosgo/session"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/yyds/errors"
	"github.com/hwcer/yyds/options"
)

// 接口权限判定 必须注册所有 options.OAuthType

var Authorize = authorizeManager{}

func init() {
	Authorize.Register(options.OAuthTypeNone, Authorize.OAuthTypeNone)
	Authorize.Register(options.OAuthTypeOAuth, Authorize.OAuthTypeOAuth)
	Authorize.Register(options.OAuthTypeSelect, Authorize.OAuthTypeSelect)
	Authorize.Register(options.OAuthTypeMaster, Authorize.OAuthTypeMaster)
}

type authorizeFunc func(r Request, req values.Metadata) (*session.Data, error)

type authorizeManager struct {
	dict map[int8]authorizeFunc
}

func (this *authorizeManager) Register(l int8, f authorizeFunc) {
	if this.dict == nil {
		this.dict = make(map[int8]authorizeFunc)
	}
	this.dict[l] = f
}

func (this *authorizeManager) oauth(r Request, req values.Metadata) (p *session.Data, err error) {
	if p, err = r.Data(); err != nil {
		return nil, values.Parse(err)
	} else if p == nil {
		return nil, errors.ErrLogin
	}
	p.KeepAlive()
	return
}

// OAuthTypeNone 普通接口
func (this *authorizeManager) OAuthTypeNone(r Request, req values.Metadata) (*session.Data, error) {
	return nil, nil
}

// OAuthTypeOAuth 账号登录
func (this *authorizeManager) OAuthTypeOAuth(r Request, req values.Metadata) (p *session.Data, err error) {
	if p, err = this.oauth(r, req); err != nil {
		return nil, err
	}
	if uuid := p.UUID(); uuid == "" {
		return nil, errors.ErrLogin
	} else {
		req[options.ServiceMetadataGUID] = p.UUID()
	}
	return
}

// OAuthTypeSelect 必须选择角色
func (this *authorizeManager) OAuthTypeSelect(r Request, req values.Metadata) (p *session.Data, err error) {
	if p, err = this.oauth(r, req); err != nil {
		return nil, err
	}
	if uid := p.GetString(options.ServiceMetadataUID); uid == "" {
		return nil, errors.ErrNotSelectRole
	} else {
		req[options.ServiceMetadataUID] = p.GetString(options.ServiceMetadataUID)
	}
	return
}

// OAuthTypeMaster 必须是GM身份
func (this *authorizeManager) OAuthTypeMaster(r Request, req values.Metadata) (p *session.Data, err error) {
	if p, err = this.OAuthTypeSelect(r, req); err != nil {
		return nil, err
	}
	if gm := p.GetInt32(options.ServiceMetadataMaster); gm == 0 {
		err = errors.ErrNeedGameMaster
	}
	return
}
