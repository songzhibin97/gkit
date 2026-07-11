package lock_redis

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/locker"
	"github.com/songzhibin97/gkit/options"
)

var (
	ErrLockFailed   = errors.New("获取锁失败")
	ErrUnLockFailed = errors.New("释放锁失败")
	ErrRenewFailed  = errors.New("续约锁失败")
	CMDLock         = `return redis.call("SET", KEYS[1], ARGV[1], "NX", "PX", ARGV[2])`
	CMDRenew        = `if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
    return 0
end`
	CMDUnlock = `if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end`
)

type Lock struct {
	config
	client redis.UniversalClient
}

func (l *Lock) lock(ctx context.Context, key string, expire int, mark string) error {
	resp, err := l.client.Eval(ctx, CMDLock, []string{key}, []string{mark, strconv.Itoa(expire)}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if errors.Is(err, redis.Nil) || resp == nil {
		return ErrLockFailed
	}
	reply, ok := resp.(string)
	if !ok || reply != "OK" {
		return ErrLockFailed
	}
	return nil
}

func (l *Lock) Lock(key string, expire int, mark string) error {
	return l.LockContext(l.client.Context(), key, expire, mark)
}

func (l *Lock) LockContext(ctx context.Context, key string, expire int, mark string) error {
	var err error
	for i := 0; i < l.retries+1; i++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		err = l.lock(ctx, key, expire, mark)
		if err == nil {
			return nil
		}
		if l.interval > 0 && i < l.retries {
			timer := time.NewTimer(l.interval)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return ctx.Err()
			}
		}
	}
	return err
}

func (l *Lock) UnLock(key string, mark string) error {
	return l.UnlockContext(l.client.Context(), key, mark)
}

func (l *Lock) UnlockContext(ctx context.Context, key string, mark string) error {
	resp, err := l.client.Eval(ctx, CMDUnlock, []string{key}, []string{mark}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if errors.Is(err, redis.Nil) || resp == nil {
		return ErrUnLockFailed
	}
	reply, ok := resp.(int64)
	if !ok || reply != 1 {
		return ErrUnLockFailed
	}
	return nil
}

func (l *Lock) Renew(key string, expire int, mark string) error {
	ctx := l.client.Context()
	resp, err := l.client.Eval(ctx, CMDRenew, []string{key}, []string{mark, strconv.Itoa(expire)}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if errors.Is(err, redis.Nil) || resp == nil {
		return ErrRenewFailed
	}
	reply, ok := resp.(int64)
	if !ok || reply != 1 {
		return ErrRenewFailed
	}
	return nil
}

func NewRedisLock(client redis.UniversalClient, opts ...options.Option) locker.Locker {
	o := &config{
		interval: 0,
		retries:  0,
	}
	for _, opt := range opts {
		opt(o)
	}
	// 如果 interval < 0, 则禁用重试
	if o.interval < 0 {
		o.interval = 0
		o.retries = 0
	}
	// 如果 retries > 0 但 interval 未设置, 使用默认值
	if o.retries > 0 && o.interval == 0 {
		o.interval = 100 * time.Millisecond
	}
	return &Lock{
		client: client,
		config: *o,
	}
}
