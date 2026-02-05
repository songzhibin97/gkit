package lock_redis

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

	// maxRetry 续约最大失败次数，超过则自动释放锁，避免死锁
	// 默认 3 次，-1 表示无限重试
	maxRetry int
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

func SetLeaseMaxRetry(maxRetry int) options.Option {
	return func(c interface{}) {
		c.(*leaseConfig).maxRetry = maxRetry
	}
}

func LeaseLock(lock locker.Locker, key string, expire int, ops ...options.Option) (func() error, error) {
	c := leaseConfig{
		enable:    true,
		interval:  time.Duration(expire) * time.Millisecond / 3, // expire单位是毫秒
		logger:    log.DefaultLogger,
		randomNum: 6,
		maxRetry:  3,
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
			retryCount := 0
			for {
				select {
				case <-ctx.Done():
					return
				case <-click.C:
					if cls.Load() {
						return
					}
					// 续约
					err := lock.Renew(key, expire, mark)
					if err != nil {
						retryCount++
						if c.maxRetry >= 0 && retryCount >= c.maxRetry {
							if c.logger != nil {
								_ = c.logger.Log(log.LevelError, "key", key, "msg", "lease retry exceeded, auto unlocking", "retryCount", retryCount)
							}
							// 自动释放锁，避免死锁
							_ = lock.UnLock(key, mark)
							cls.Store(true)
							return
						}
						if c.logger != nil {
							_ = c.logger.Log(log.LevelError, "key", key, "err", err, "retryCount", retryCount)
						}
					} else {
						// 续约成功，重置计数
						retryCount = 0
					}
				}
			}
		}()
	}

	return func() error {
		if cls.Load() {
			return nil
		}
		cls.Store(true)
		defer cancel()
		return lock.UnLock(key, mark)
	}, nil
}
