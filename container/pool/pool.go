package pool

import (
	"context"
	"errors"
	"time"

	"github.com/songzhibin97/gkit/options"
)

// package pool: 连接池
const (
	minDuration = 100 * time.Millisecond
)

var (
	ErrPoolNewFuncIsNull = errors.New("container/pool: 初始化函数为空")
	// ErrPoolExhausted 连接以耗尽
	ErrPoolExhausted = errors.New("container/pool: 连接已耗尽")
	// ErrPoolClosed 连接池已关闭.
	ErrPoolClosed = errors.New("container/pool: 连接池已关闭")

	// nowFunc: 返回当前时间
	nowFunc = time.Now
)

type IShutdown interface {
	Shutdown() error
}

// Pool interface.
type Pool interface {
	New(f func(ctx context.Context) (IShutdown, error))
	Get(ctx context.Context) (IShutdown, error)
	Put(ctx context.Context, c IShutdown, forceClose bool) error
	Shutdown() error
}

// config Pool 选项
type config struct {
	// active: 池中的连接数, 如果为 == 0 则无限制
	active uint64

	// idle 最大空闲数
	idle uint64

	// idleTimeout 空闲等待的时间
	idleTimeout time.Duration

	// waitTimeout 如果设置 waitTimeout 如果池内资源已经耗尽,将会等待 time.Duration 时间, 直到某个连接退回
	waitTimeout time.Duration

	// wait 如果是 true 则等待 waitTimeout 时间, 否则无线傻等
	wait bool
}

// item:
type item struct {
	createdAt time.Time
	s         IShutdown
}

// expire 是否到期
func (i *item) expire(timeout time.Duration) bool {
	if timeout <= 0 {
		return false
	}
	return i.createdAt.Add(timeout).Before(nowFunc())
}

// shutdown 关闭
func (i *item) shutdown() error {
	return i.s.Shutdown()
}

// defaultConfig 默认配置
func defaultConfig() *config {
	return &config{
		active:      20,
		idle:        10,
		idleTimeout: 90 * time.Second,
		waitTimeout: 0,
		wait:        false,
	}
}

// Option选项

// SetActive 设置 Pool 连接数, 如果 == 0 则无限制
func SetActive(active uint64) options.Option {
	return func(c interface{}) {
		c.(*config).active = active
	}
}

// SetIdle 设置最大空闲连接数
func SetIdle(idle uint64) options.Option {
	return func(c interface{}) {
		c.(*config).idle = idle
	}
}

// SetIdleTimeout 设置空闲等待时间
func SetIdleTimeout(idleTimeout time.Duration) options.Option {
	return func(c interface{}) {
		c.(*config).idleTimeout = idleTimeout
	}
}

// SetWait 设置期望等待
func SetWait(wait bool, waitTimeout time.Duration) options.Option {
	return func(c interface{}) {
		conf := c.(*config)
		conf.wait = wait
		conf.waitTimeout = waitTimeout
	}
}
