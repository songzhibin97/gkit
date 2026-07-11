package egroup

import (
	"context"
	"errors"
	"sync"

	"github.com/songzhibin97/gkit/options"

	"github.com/songzhibin97/gkit/goroutine"
)

type Group struct {
	ctx     context.Context
	cancel  func()
	wg      sync.WaitGroup
	stateMu sync.Mutex
	closed  bool
	sync.Once
	goroutine goroutine.GGroup
	err       error
}

func WithContextGroup(ctx context.Context, group goroutine.GGroup) *Group {
	g := &Group{}
	g.ctx, g.cancel = context.WithCancel(ctx)
	g.goroutine = group
	return g
}

// WithContext 实例化方法
// 传入 NewGoroutine Option
func WithContext(ctx context.Context, opts ...options.Option) *Group {
	g := &Group{}
	g.ctx, g.cancel = context.WithCancel(ctx)
	g.goroutine = goroutine.NewGoroutine(g.ctx, opts...)
	return g
}

var ErrGroupClosed = errors.New("group has closed")

// Wait 等待
func (g *Group) Wait() error {
	g.stateMu.Lock()
	closed := g.closed
	g.stateMu.Unlock()
	if closed {
		return ErrGroupClosed
	}

	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}

func (g *Group) Shutdown() error {
	g.stateMu.Lock()
	if g.closed {
		g.stateMu.Unlock()
		return nil
	}
	g.closed = true
	if g.cancel != nil {
		g.cancel()
	}
	g.stateMu.Unlock()

	g.wg.Wait()
	return g.goroutine.Shutdown()
}

// Go 异步调用
func (g *Group) Go(f func() error) {
	g.stateMu.Lock()
	if g.closed {
		g.stateMu.Unlock()
		return
	}
	g.wg.Add(1)
	g.stateMu.Unlock()

	ok := g.goroutine.AddTaskN(g.ctx, func() {
		defer g.wg.Done()
		if err := f(); err != nil {
			g.recordError(err)
		}
	})
	if !ok {
		g.recordError(ErrGroupClosed)
		g.wg.Done()
	}
}

func (g *Group) recordError(err error) {
	if err == nil {
		return
	}
	g.Do(func() {
		g.err = err
		if g.cancel != nil {
			g.cancel()
		}
	})
}
