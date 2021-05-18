package local_cache

import (
	"github.com/songzhibin97/gkit/options"
	"time"
)

type Config struct {
	// defaultExpire 默认超时时间
	defaultExpire time.Duration

	// interval 间隔时间
	interval time.Duration
	// fn 哨兵周期执行的函数
	fn func()

	// capture 捕获删除对象时间 会返回kv值用于用户自定义处理
	capture func(k string, v interface{})
}

// SetInternal 设置间隔时间
func SetInternal(interval time.Duration) options.Option {
	return func(c interface{}) {
		c.(*Config).interval = interval
	}
}

func SetDefaultExpire(expire time.Duration) options.Option {
	return func(c interface{}) {
		c.(*Config).defaultExpire = expire
	}
}

func SetFn(fn func()) options.Option {
	return func(c interface{}) {
		c.(*Config).fn = fn
	}
}

func SetCapture(capture func(k string, v interface{})) options.Option {
	return func(c interface{}) {
		c.(*Config).capture = capture
	}
}
