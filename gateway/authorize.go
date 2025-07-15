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

var Authorize = authorizeManager{}

func init() {
	Authorize.Register(options.OAuthTypeNone, Authorize.OAuthTypeNone)
	Authorize.Register(options.OAuthTypeOAuth, Authorize.OAuthTypeOAuth)
	Authorize.Register(options.OAuthTypeSelect, Authorize.OAuthTypeSelect)
}

type RequestSocket interface {
	Socket() *cosnet.Socket
}

type authorizeFunc func(r Request, req values.Metadata, isMaster bool) (*session.Data, error)

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
func (this *authorizeManager) OAuthTypeNone(r Request, req values.Metadata, isMaster bool) (p *session.Data, err error) {
	if p, _ = r.Data(); p != nil {
		p.KeepAlive()
	}
	if f, ok := r.(RequestSocket); ok {
		sock := f.Socket()
		req[options.ServiceSocketId] = fmt.Sprintf("%d", sock.Id())
	}
	return
}

// OAuthTypeOAuth 账号登录
func (this *authorizeManager) OAuthTypeOAuth(r Request, req values.Metadata, isMaster bool) (p *session.Data, err error) {
	if p, err = this.oauth(r, req); err != nil {
		return nil, err
	}
	if uuid := p.UUID(); uuid == "" {
		return nil, errors.ErrLogin
	} else {
		req[options.ServiceMetadataGUID] = uuid
	}
	if isMaster {
		err = this.IsMaster(p)
	}
	return
}

// OAuthTypeSelect 必须选择角色
func (this *authorizeManager) OAuthTypeSelect(r Request, req values.Metadata, isMaster bool) (p *session.Data, err error) {
	if p, err = this.oauth(r, req); err != nil {
		return nil, err
	}
	if uid := p.GetString(options.ServiceMetadataUID); uid == "" {
		return nil, errors.ErrNotSelectRole
	} else {
		req[options.ServiceMetadataUID] = p.GetString(options.ServiceMetadataUID)
	}
	if isMaster {
		err = this.IsMaster(p)
	}
	return
}

// IsMaster 是GM
func (this *authorizeManager) IsMaster(p *session.Data) (err error) {
	if p == nil {
		return errors.ErrNeedGameMaster
	}
	if gm := p.GetInt32(options.ServiceMetadataMaster); gm == 0 {
		err = errors.ErrNeedGameMaster
	}
	return
}
