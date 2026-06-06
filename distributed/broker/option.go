package broker

import (
	"context"

	"github.com/songzhibin97/gkit/options"
)

func SetRetry(retry bool) options.Option {
	return func(c interface{}) {
		c.(*Broker).SetRetry(retry)
	}
}

func SetRetryFn(fn func(ctx context.Context)) options.Option {
	return func(c interface{}) {
		c.(*Broker).retryFn = fn
	}
}
