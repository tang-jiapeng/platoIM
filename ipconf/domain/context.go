package domain

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

type IpConfContext struct {
	Ctx       *context.Context
	AppCtx    *app.RequestContext
	ClientCtx *ClientCtx
}

type ClientCtx struct {
	IP string `json:"ip"`
}

func BuildIpConfContext(c *context.Context, ctx *app.RequestContext) *IpConfContext {
	ipConfContext := &IpConfContext{
		Ctx:       c,
		AppCtx:    ctx,
		ClientCtx: &ClientCtx{},
	}
	return ipConfContext
}
