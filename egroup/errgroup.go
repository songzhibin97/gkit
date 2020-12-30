package egroup

import (
	"context"
	"sync"
)

// Group
type Group struct {
	ctx    context.Context
	cancel func()
	wg     sync.WaitGroup
	sync.Once
	err error
}

// WithContext: 实例化方法
func WithContext(ctx context.Context) *Group {
	g := &Group{}
	g.ctx, g.cancel = context.WithCancel(ctx)
	return g
}

// wait: 等待
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}

// Go: 异步调用
func (g *Group) Go(f func() error) {
	g.wg.Add(1)
	go func() {
		// 兜底
		defer func() {
			if err := recover(); err != nil {
				return
			}
		}()
		defer g.wg.Done()
		if err := f(); err != nil {
			g.Once.Do(func() {
				// 级联取消
				g.err = err
				if g.cancel != nil {
					g.cancel()
				}
			})
		}
	}()
}
