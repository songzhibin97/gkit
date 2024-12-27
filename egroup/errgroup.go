package egroup

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/songzhibin97/gkit/options"

	"github.com/songzhibin97/gkit/goroutine"
)

type Group struct {
	ctx    context.Context
	cancel func()
	wg     sync.WaitGroup
	sync.Once
	goroutine goroutine.GGroup
	err       error
	close     int32
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
	g.goroutine = goroutine.NewGoroutine(ctx, opts...)
	return g
}

var ErrGroupClosed = errors.New("group has closed")

// Wait 等待
func (g *Group) Wait() error {
	if atomic.LoadInt32(&g.close) != 0 {
		return ErrGroupClosed
	}

	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}

func (g *Group) Shutdown() error {
	if atomic.CompareAndSwapInt32(&g.close, 0, 1) {
		return nil
	}
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.goroutine.Shutdown()
}

// Go 异步调用
func (g *Group) Go(f func() error) {
	if atomic.LoadInt32(&g.close) != 0 {
		return
	}
	g.wg.Add(1)
	g.goroutine.AddTask(func() {
		defer g.wg.Done()
		if err := f(); err != nil {
			g.Do(func() {
				// 级联取消
				g.err = err
				if g.cancel != nil {
					g.cancel()
				}
			})
		}
	})
}
