package appointmentbooking

import (
	stdlib "context"
	"time"
	"github.com/tinywasm/context"
)

func ToStd(ctx *context.Context) stdlib.Context {
	return &stdCtx{ctx: ctx}
}

type stdCtx struct {
	ctx *context.Context
}

func (c *stdCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *stdCtx) Done() <-chan struct{}           { return nil }
func (c *stdCtx) Err() error                      { return nil }
func (c *stdCtx) Value(key any) any {
	if k, ok := key.(string); ok {
		return c.ctx.Value(k)
	}
	return nil
}
