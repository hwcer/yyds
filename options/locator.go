package options

import (
	"context"

	"github.com/hwcer/cosrpc"
	"github.com/hwcer/cosrpc/client"
)

var Locator = &locator{}

type locator struct {
}

func (l *locator) Enabled() bool {
	return cosrpc.Service.Get(ServiceTypeLocator) != ""
}

func (l *locator) Call(ctx context.Context, serviceMethod string, args, reply any) (err error) {
	if !l.Enabled() {
		return nil
	}
	return client.XCall(ctx, ServiceTypeLocator, serviceMethod, args, reply)
}

func (l *locator) Async(ctx context.Context, serviceMethod string, args any) (call *client.Caller, err error) {
	if !l.Enabled() {
		return nil, nil
	}
	call, err = client.Async(ctx, ServiceTypeLocator, serviceMethod, args)
	return nil, err
}
