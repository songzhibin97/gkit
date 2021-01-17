package pool

import (
	"context"
	"errors"
	"time"
)

// package pool: 连接池

var (
	// ErrPoolExhausted: 连接以耗尽
	ErrPoolExhausted = errors.New("container/pool: 连接已耗尽")
	// ErrPoolClosed: 连接池已关闭.
	ErrPoolClosed = errors.New("container/pool: 连接池已关闭")

	// nowFunc: 返回当前时间
	nowFunc = time.Now
)

type Shutdown interface {
	Shutdown() error
}

// Pool interface.
type Pool interface {
	Get(ctx context.Context) (Shutdown, error)
	Put(ctx context.Context, c Shutdown, forceClose bool) error
	Shutdown
}

//// shutdown: 实现 Shutdown 接口 用于 mock
//type shutdown struct{}
//
//func (s *shutdown) Shutdown() error {
//	return nil
//}

// Config: Pool 选项
type Config struct {
	// Active: 池中的连接数, 如果为 <= 0 则无限制
	Active uint64

	// Idle: 最大空闲数
	Idle uint64

	// IdleTimeout: 关闭后等待 time.Duration 连接归还
	IdleTimeout time.Duration

	// WaitTimeout: 如果设置 WaitTimeout 如果池内资源已经耗尽,将会等待 time.Duration 时间, 直到某个连接退回
	WaitTimeout time.Duration

	// Wait: 如果是 true 则等待 WaitTimeout 时间, 否则无线傻等
	Wait bool
}

// item:
type item struct {
	createdAt time.Time
	s         Shutdown
}

// expire: 是否到期
func (i *item) expire(timeout time.Duration) bool {
	if timeout <= 0 {
		return false
	}
	return i.createdAt.Add(timeout).Before(nowFunc())
}

// close: 关闭
func (i *item) shutdown() error {
	return i.s.Shutdown()
}
