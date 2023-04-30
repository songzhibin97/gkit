package lock_ridis

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/distributed/locker"

	"github.com/songzhibin97/gkit/log"

	"github.com/songzhibin97/gkit/options"

	"github.com/songzhibin97/gkit/tools/rand_string"
)

// leaseConfig
type leaseConfig struct {
	// 是否开启续约,默认开启
	enable bool
	// interval: 续约时间间隔
	// 只有 retries > 0 才有效
	// interval < 0 的话 retries 同样无效
	interval time.Duration

	// randomNum 生成随机数的位数
	randomNum int

	// logger 内部错误输出
	logger log.Logger
}

func SetLeaseEnable(enable bool) options.Option {
	return func(c interface{}) {
		c.(*leaseConfig).enable = enable
	}
}

func SetLeaseInterval(duration time.Duration) options.Option {
	return func(c interface{}) {
		c.(*leaseConfig).interval = duration
	}
}

func SetLeaseRandomNum(randomNum int) options.Option {
	return func(c interface{}) {
		c.(*leaseConfig).randomNum = randomNum
	}
}

func SetLeaseLogger(logger log.Logger) options.Option {
	return func(c interface{}) {
		c.(*leaseConfig).logger = logger
	}
}

func LeaseLock(lock locker.Locker, key string, expire int, ops ...options.Option) (func() error, error) {
	c := leaseConfig{
		enable:    true,
		interval:  time.Duration(expire/1000) * time.Second / 3,
		logger:    log.DefaultLogger,
		randomNum: 6,
	}
	for _, op := range ops {
		op(&c)
	}
	if c.interval <= 0 {
		c.enable = false
	}

	mark := rand_string.RandomLetter(c.randomNum)
	err := lock.Lock(key, expire, mark)
	if err != nil {
		return nil, err
	}

	var cls atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())

	if c.enable {
		go func() {
			click := time.NewTicker(c.interval)
			defer click.Stop()
			for {
				select {
				case <-ctx.Done():
					cancel()
					return
				case <-click.C:
					if cls.Load() {
						return
					}
					// 续约
					err = lock.Lock(key, expire, mark)
					if err != nil && c.logger != nil {
						_ = c.logger.Log(log.LevelError, "key", key, "err", err)
					}
				}
			}
		}()
	}

	return func() error {
		cls.Store(true)
		cancel()
		return lock.UnLock(key, mark)
	}, nil
}
