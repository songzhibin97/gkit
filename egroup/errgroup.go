package egroup

import (
	"context"
	"sync"

	"github.com/songzhibin97/gkit/goroutine"
)

type Group struct {
	ctx    context.Context
	cancel func()
	wg     sync.WaitGroup
	sync.Once
	goroutine goroutine.GGroup
	err       error
}

// WithContext 实例化方法
func WithContext(ctx context.Context) *Group {
	g := &Group{}
	g.ctx, g.cancel = context.WithCancel(ctx)
	g.goroutine = goroutine.NewGoroutine(ctx)
	return g
}

// Wait 等待
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}

// Go 异步调用
func (g *Group) Go(f func() error) {
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
