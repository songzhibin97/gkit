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
			completed := false
			// Do not use recover() to detect this state: on Go 1.20 a
			// panic(nil) is indistinguishable from no panic by its recovered
			// value. A normal-completion flag lets every panic propagate while
			// the defer still releases the in-flight slot as Drop.
			defer func() {
				if !completed {
					f(overload.DoneInfo{Op: overload.Drop})
					return
				}
				// A normally-returning handler completed real work even when it
				// reports a business error. Preserve the configured operation so
				// only fail-fast or an explicit Drop skips success stats.
				f(overload.DoneInfo{Op: defaultOp})
			}()
			resp, err = next(ctx, i)
			completed = true
			return resp, err
		}
	}
}
