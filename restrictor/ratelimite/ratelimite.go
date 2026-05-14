package ratelimite

import (
	"context"
	"errors"
	"time"

	"github.com/juju/ratelimit"
	"github.com/songzhibin97/gkit/restrictor"
)

var ErrTimeOut = errors.New("restrictor/ratelimit: 超时")

// package ratelimite: https://pkg.go.dev/github.com/juju/ratelimit 实现 limiter 接口

// defaultWaitWhenNoDeadline bounds Wait when the caller's context has no
// deadline. Previously the helper baked in 100ms with no documentation;
// preserving the floor as a named constant makes the behaviour explicit.
const defaultWaitWhenNoDeadline = 100 * time.Millisecond

func NewRateLimit(bucket *ratelimit.Bucket) (restrictor.AllowFunc, restrictor.WaitFunc) {
	return func(now time.Time, n int) bool {
			// TakeMaxDuration(n, 0) returns (0, true) only when n tokens are
			// immediately available; on false it does NOT consume tokens.
			// The previous TakeAvailable(n) >= n check consumed up to n
			// tokens whether or not the call returned true, leaking tokens
			// on every partial-denial.
			_, ok := bucket.TakeMaxDuration(int64(n), 0)
			return ok
		},
		func(ctx context.Context, n int) error {
			var maxWait time.Duration
			if d, ok := ctx.Deadline(); ok {
				maxWait = time.Until(d)
				if maxWait <= 0 {
					return ErrTimeOut
				}
			} else {
				maxWait = defaultWaitWhenNoDeadline
			}
			d, ok := bucket.TakeMaxDuration(int64(n), maxWait)
			if !ok {
				return ErrTimeOut
			}
			if d <= 0 {
				return nil
			}
			// Sleep until either the bucket allows us through or the ctx is
			// cancelled. juju/ratelimit's WaitMaxDuration ignores ctx, so
			// the previous wrapper held callers past Cancel/Deadline.
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-t.C:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
}
