package ratelimite

import (
	"context"
	"errors"
	"time"

	"github.com/juju/ratelimit"
	"github.com/songzhibin97/gkit/restrictor"
)

var ErrTimeOut = errors.New("restrictor/ratelimite: 超时")

// package ratelimite: https://pkg.go.dev/github.com/juju/ratelimit 实现 limiter 接口

func NewRateLimit(bucket *ratelimit.Bucket) (restrictor.AllowFunc, restrictor.WaitFunc) {
	return func(now time.Time, n int) bool {
			return bucket.TakeAvailable(int64(n)) >= int64(n)
		},
		func(ctx context.Context, n int) error {
			// 获取超时时间
			if d, ok := ctx.Deadline(); ok {
				if !bucket.WaitMaxDuration(int64(n), time.Until(d)) {
					return ErrTimeOut
				}
				return nil
			}
			// 表示context没有设置超时时间
			if bucket.WaitMaxDuration(int64(n), 100*time.Millisecond) {
				return ErrTimeOut
			}
			return nil
		}
}
