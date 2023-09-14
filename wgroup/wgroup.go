package wgroup

import (
	"context"
	"sync"

	"github.com/songzhibin97/gkit/goroutine"
)

type Group struct {
	wg        sync.WaitGroup
	goroutine goroutine.GGroup
}

func (g *Group) Wait() {
	g.wg.Wait()
}

func (g *Group) AddTask(f func()) bool {
	g.wg.Add(1)
	return g.goroutine.AddTask(func() {
		defer g.wg.Done()
		f()
	})
}

func (g *Group) AddTaskN(ctx context.Context, f func()) bool {
	g.wg.Add(1)
	return g.goroutine.AddTaskN(ctx, func() {
		defer g.wg.Done()
		f()
	})
}

func WithContextGroup(group goroutine.GGroup) *Group {
	g := &Group{}
	g.goroutine = group
	return g
}

// WithContext 实例化方法
func WithContext(ctx context.Context) *Group {
	g := &Group{}
	g.goroutine = goroutine.NewGoroutine(ctx)
	return g
}
