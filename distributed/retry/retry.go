package retry

import (
	"context"
	"time"
)

func Retry() func(ctx context.Context) {
	retryIn := 0
	fibonacci := Fibonacci()
	return func(ctx context.Context) {
		if retryIn > 0 {
			select {
			case <-ctx.Done():
				break
			case <-time.After(time.Second * time.Duration(retryIn)):
				break
			}
		}
		retryIn = fibonacci()
	}
}
