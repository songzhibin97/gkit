package bbr

import (
	"context"

	"github.com/songzhibin97/gkit/middleware"
	"github.com/songzhibin97/gkit/options"
	"github.com/songzhibin97/gkit/overload"
)

const (
	LimitKey = "LimitKey"
	LimitOp  = "LimitLoad"
)

func NewLimiter(options ...options.Option) middleware.MiddleWare {
	return newLimiterWithGroup(NewGroup(options...))
}

// newLimiterWithGroup builds the limiter middleware around a caller-provided
// Group, so tests can inspect the same Group's per-key limiter (e.g. its
// inFlight) that the middleware drives.
func newLimiterWithGroup(g *Group) middleware.MiddleWare {
	return func(next middleware.Endpoint) middleware.Endpoint {
		return func(ctx context.Context, i interface{}) (resp interface{}, err error) {
			defaultKey := "default"
			defaultOp := overload.Success
			if v := ctx.Value(LimitKey); v != nil {
				defaultKey = v.(string)
			}
			if v := ctx.Value(LimitOp); v != nil {
				defaultOp = v.(overload.Op)
			}
			limiter := g.Get(defaultKey)
			f, allowErr := limiter.Allow(ctx)
			if allowErr != nil {
				return nil, allowErr
			}
			// Always release the inFlight slot, even if next panics. The
			// previous code called f(...) inline after `next` returned, so a
			// panic in any downstream middleware leaked the slot permanently
			// — eventually inFlight > maxFlight for that key and every
			// request to it was dropped.
			defer func() {
				if r := recover(); r != nil {
					f(overload.DoneInfo{Op: overload.Drop})
					panic(r)
				}
				if err != nil {
					f(overload.DoneInfo{Op: overload.Drop})
				} else {
					f(overload.DoneInfo{Op: defaultOp})
				}
			}()
			resp, err = next(ctx, i)
			return resp, err
		}
	}
}
