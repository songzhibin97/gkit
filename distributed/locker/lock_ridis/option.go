package lock_ridis

import (
	"time"

	"github.com/songzhibin97/gkit/options"
)

// config
type config struct {
	// interval: 重试间隔时间
	// 只有 retries > 0 才有效
	// interval < 0 的话 retries 同样无效
	interval time.Duration

	// retries间隔次数
	// retries > 0
	retries int
}

// SetInterval 设置重试间隔时间
func SetInterval(duration time.Duration) options.Option {
	return func(c interface{}) {
		c.(*config).interval = duration
	}
}

// SetRetries 设置重试次数
func SetRetries(retries int) options.Option {
	return func(c interface{}) {
		c.(*config).retries = retries
	}
}
