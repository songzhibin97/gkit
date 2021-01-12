package window

import (
	"context"
	"sync"
	"time"
)

// 选项模式
type Option func(conf *Conf)

// SetSize: 设置大小
func SetSize(size uint) Option {
	return func(conf *Conf) {
		conf.size = size
	}
}

// SetInterval: 设置间隔时间
func SetInterval(interval time.Duration) Option {
	return func(conf *Conf) {
		conf.interval = interval
	}
}

// SetContext: 设置context
func SetContext(c context.Context) Option {
	return func(conf *Conf) {
		conf.ctx = c
	}
}

// InitWindow: 实例化
func InitWindow(options ...Option) Windower {
	w := Window{
		// 默认值:
		Conf: Conf{
			size:     5,
			interval: time.Second,
			ctx:      context.Background(),
		},
	}
	for _, option := range options {
		option(&w.Conf)
	}
	w.buffer = make([]map[string]uint, w.size)
	w.buffer[0] = make(map[string]uint)
	w.total = make(map[string]uint)
	w.bufLock = make([]sync.Mutex, w.size)
	w.ctx, w.cancel = context.WithCancel(w.ctx)
	// 开启哨兵
	go w.Sentinel()
	return &w
}
