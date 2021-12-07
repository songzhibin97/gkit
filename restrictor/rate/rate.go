package rate

// package rate: https://pkg.go.dev/golang.org/x/time/rate 实现 limiter 接口

import (
	"context"
	"time"

	"github.com/songzhibin97/gkit/restrictor"
	"golang.org/x/time/rate"
)

// NewRate 返回limiter对应的 restrictor.AllowFunc, restrictor.WaitFunc
func NewRate(limiter *rate.Limiter) (restrictor.AllowFunc, restrictor.WaitFunc) {
	return func(now time.Time, n int) bool {
			return limiter.AllowN(now, n)
		}, func(ctx context.Context, n int) error {
			return limiter.WaitN(ctx, n)
		}
}
