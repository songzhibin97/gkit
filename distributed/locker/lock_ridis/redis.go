package lock_ridis

import (
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
	CMDLock         = `if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
    return "OK"
else
    return redis.call("SET", KEYS[1], ARGV[1], "NX", "PX", ARGV[2])
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

func (l *Lock) lock(key string, expire int, mark string) error {
	ctx := l.client.Context()
	resp, err := l.client.Eval(ctx, CMDLock, []string{key}, []string{mark, strconv.Itoa(expire)}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if errors.Is(err, redis.Nil) || resp == nil {
		return ErrLockFailed
	}
	reply, ok := resp.(string)
	if !ok || reply != "OK" {
		return ErrUnLockFailed
	}
	return nil
}

func (l *Lock) Lock(key string, expire int, mark string) error {
	var err error
	for i := 0; i < l.retries+1; i++ {
		err = l.lock(key, expire, mark)
		if err == nil {
			break
		}
		if l.interval > 0 {
			time.Sleep(l.interval)
		}
	}
	return err
}

func (l *Lock) UnLock(key string, mark string) error {
	ctx := l.client.Context()
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

func NewRedisLock(client redis.UniversalClient, opts ...options.Option) locker.Locker {
	o := &config{
		interval: 0,
		retries:  0,
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.interval <= 0 || o.retries <= 0 {
		o.interval = 0
		o.retries = 0
	}
	return &Lock{
		client: client,
		config: *o,
	}
}
