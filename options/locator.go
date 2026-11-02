package options

import (
	"context"

	"github.com/hwcer/cosrpc"
	"github.com/hwcer/cosrpc/client"
	"github.com/hwcer/gateway/gwcfg"
)

var Locator = &locator{}

type locator struct {
}

func (l *locator) Enabled() bool {
	return cosrpc.Service.Get(gwcfg.ServiceTypeLocator) != ""
}

func (l *locator) Call(ctx context.Context, serviceMethod string, args, reply any) (err error) {
	if !l.Enabled() {
		return nil
	}
	return client.XCall(ctx, gwcfg.ServiceTypeLocator, serviceMethod, args, reply)
}

func (l *locator) Async(ctx context.Context, serviceMethod string, args any) (call *client.Caller, err error) {
	if !l.Enabled() {
		return nil, nil
	}
	call, err = client.Async(ctx, gwcfg.ServiceTypeLocator, serviceMethod, args)
	return nil, err
}
