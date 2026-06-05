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

// Lease is a renewable distributed lock acquired by NewLease. Callers must
// either invoke Cancel to release voluntarily, or watch Lost() to detect
// that the renew goroutine has given up — the latter is the only safe way
// to learn that another holder may now own the key.
type Lease struct {
	cancel func() error
	lost   chan struct{}
}

// Cancel releases the lock and stops the renew goroutine. Returns the
// underlying UnLock error, or nil if the lease was already lost.
func (l *Lease) Cancel() error { return l.cancel() }

// Lost returns a channel that is closed when the renew goroutine has
// permanently given up renewing the lease (retries exhausted). After Lost
// fires, the caller MUST stop treating the protected section as held — the
// underlying key may have already been acquired by another holder.
func (l *Lease) Lost() <-chan struct{} { return l.lost }

// LeaseLock acquires a renewable lock and returns a closure that releases
// it.
//
// Deprecated: the returned closure cannot signal lease loss. After the
// renew goroutine's max retries are exhausted it silently exits and the
// closure becomes a no-op — the application keeps running its critical
// section while another holder may have taken the key. Use NewLease, which
// exposes a Lost() channel.
func LeaseLock(lock locker.Locker, key string, expire int, ops ...options.Option) (func() error, error) {
	lease, err := NewLease(lock, key, expire, ops...)
	if err != nil {
		return nil, err
	}
	return lease.Cancel, nil
}

// NewLease acquires a renewable distributed lock and returns a *Lease that
// the caller can Cancel and whose Lost() channel signals permanent renew
// failure.
func NewLease(lock locker.Locker, key string, expire int, ops ...options.Option) (*Lease, error) {
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
	ctx, cancelCtx := context.WithCancel(context.Background())
	lost := make(chan struct{})

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
					err := lock.Renew(key, expire, mark)
					if err != nil {
						retryCount++
						if c.maxRetry >= 0 && retryCount >= c.maxRetry {
							if c.logger != nil {
								_ = c.logger.Log(log.LevelError, "key", key, "msg", "lease retry exceeded, auto unlocking", "retryCount", retryCount)
							}
							_ = lock.UnLock(key, mark)
							// Signal lost BEFORE setting cls so observers can
							// distinguish "we cancelled" from "we lost".
							close(lost)
							cls.Store(true)
							return
						}
						if c.logger != nil {
							_ = c.logger.Log(log.LevelError, "key", key, "err", err, "retryCount", retryCount)
						}
					} else {
						retryCount = 0
					}
				}
			}
		}()
	}

	return &Lease{
		lost: lost,
		cancel: func() error {
			if cls.Load() {
				return nil
			}
			cls.Store(true)
			cancelCtx()
			return lock.UnLock(key, mark)
		},
	}, nil
}
