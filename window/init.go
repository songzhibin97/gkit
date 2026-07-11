package window

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/options"
)

const (
	defaultWindowSize     uint          = 5
	defaultWindowInterval time.Duration = time.Second
)

// SetSize 设置大小。零值会由 NewWindow 回落到默认大小。
func SetSize(size uint) options.Option {
	return func(c interface{}) {
		c.(*conf).size = size
	}
}

// SetInterval 设置间隔时间。非正值会由 NewWindow 回落到默认间隔。
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
			size:     defaultWindowSize,
			interval: defaultWindowInterval,
			ctx:      context.Background(),
		},
	}
	for _, option := range options {
		option(&w.conf)
	}
	if w.size == 0 {
		w.size = defaultWindowSize
	}
	if w.interval <= 0 {
		w.interval = defaultWindowInterval
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
