package bbr

import (
	"Songzhibin/GKit/middleware"
	"Songzhibin/GKit/overload"
	"context"
)

func NewLimiter(conf *Config) middleware.MiddleWare {
	limiter := newLimiter(conf)
	return func(next middleware.Endpoint) middleware.Endpoint {
		return func(ctx context.Context, i interface{}) (interface{}, error) {
			if f, err := limiter.Allow(ctx); err != nil {
				return nil, err
			} else {
				f(overload.DoneInfo{Op: overload.Success})
				return next(ctx, i)
			}
		}
	}
}
