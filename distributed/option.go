package distributed

import "github.com/songzhibin97/gkit/options"

// SetNoUnixSignals 设置是否优雅关闭
func SetNoUnixSignals(noUnixSignals bool) options.Option {
	return func(o interface{}) {
		o.(*Config).NoUnixSignals = noUnixSignals
	}
}

// SetResultExpire 设置 backend result 过期时间
func SetResultExpire(expire int64) options.Option {
	return func(o interface{}) {
		o.(*Config).ResultExpire = expire
	}
}

// SetConcurrency 设置并发量
func SetConcurrency(concurrency int64) options.Option {
	return func(o interface{}) {
		o.(*Config).Concurrency = concurrency
	}
}

// SetConsumeQueue 设置消费队列
func SetConsumeQueue(consumeQueue string) options.Option {
	return func(o interface{}) {
		o.(*Config).ConsumeQueue = consumeQueue
	}
}

// SetDelayedQueue 设置延时队列
func SetDelayedQueue(delayedQueue string) options.Option {
	return func(o interface{}) {
		o.(*Config).DelayedQueue = delayedQueue
	}
}
