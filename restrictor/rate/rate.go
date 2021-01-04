package rate

// package rate: https://pkg.go.dev/golang.org/x/time/rate 实现 limiter 接口

import (
	"Songzhibin/GKit/restrictor"
	"context"
	"golang.org/x/time/rate"
	"time"
)

// NewRate: 返回limiter对应的 restrictor.AllowFunc, restrictor.WaitFunc
func NewRate(limiter *rate.Limiter) (restrictor.AllowFunc, restrictor.WaitFunc) {
	return func(now time.Time, n int) bool {
			return limiter.AllowN(now, n)
		}, func(ctx context.Context, n int) error {
			return limiter.WaitN(ctx, n)
		}
}


