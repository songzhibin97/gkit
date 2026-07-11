package bbr

import (
	"context"

	"github.com/songzhibin97/gkit/middleware"
	"github.com/songzhibin97/gkit/options"
	"github.com/songzhibin97/gkit/overload"
)

const (
	// LimitKey is the legacy string context key for selecting a limiter.
	//
	// Deprecated: use WithLimitKey.
	LimitKey = "LimitKey"
	// LimitOp is the legacy string context key for selecting the completion op.
	//
	// Deprecated: use WithLimitOp.
	LimitOp = "LimitLoad"
)

type contextKey uint8

const (
	limitKeyContextKey contextKey = iota
	limitOpContextKey
)

// WithLimitKey returns a child context that selects the named limiter.
func WithLimitKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, limitKeyContextKey, key)
}

// WithLimitOp returns a child context that reports op when the endpoint returns.
func WithLimitOp(ctx context.Context, op overload.Op) context.Context {
	return context.WithValue(ctx, limitOpContextKey, op)
}

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
			if v, ok := ctx.Value(limitKeyContextKey).(string); ok {
				defaultKey = v
			} else if v, ok := ctx.Value(LimitKey).(string); ok {
				defaultKey = v
			}
			if v, ok := ctx.Value(limitOpContextKey).(overload.Op); ok {
				defaultOp = v
			} else if v, ok := ctx.Value(LimitOp).(overload.Op); ok {
				defaultOp = v
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
