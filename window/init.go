package window

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/options"
)

// SetSize 设置大小
func SetSize(size uint) options.Option {
	return func(c interface{}) {
		c.(*conf).size = size
	}
}

// SetInterval 设置间隔时间
func SetInterval(interval time.Duration) options.Option {
	return func(c interface{}) {
		c.(*conf).interval = interval
	}
}

// SetContext 设置context
func SetContext(context context.Context) options.Option {
	return func(c interface{}) {
		c.(*conf).ctx = context
	}
}

// NewWindow 实例化
func NewWindow(options ...options.Option) SlidingWindow {
	w := Window{
		// 默认值:
		conf: conf{
			size:     5,
			interval: time.Second,
			ctx:      context.Background(),
		},
	}
	for _, option := range options {
		option(&w.conf)
	}
	w.buffer = make([]atomic.Value, w.size)
	for i := uint(0); i < w.size; i++ {
		w.buffer[i].Store(make(map[string]uint))
	}
	w.communication = make(chan Index, w.size)
	w.ctx, w.cancel = context.WithCancel(w.ctx)
	// 开启哨兵
	go w.sentinel()
	return &w
}
