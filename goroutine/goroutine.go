package goroutine

// package goroutine: 管理goroutine并发量托管任务以及兜底

import (
	"Songzhibin/GKit/timeout"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrRepeatClose = errors.New("goroutine/goroutine :重复关闭")
)

// Goroutine:
type Goroutine struct {
	close int32

	// n: 当前goroutine的数量
	n int64
	// 参数选项
	options
	// wait
	wait sync.WaitGroup
	// ctx context
	ctx context.Context
	// cancel
	cancel context.CancelFunc
	// task
	task chan func()
}

// NewGoroutine: 实例化方法
func NewGoroutine(ctx context.Context, opts ...Option) GGroup {
	ctx, cancel := context.WithCancel(ctx)
	o := options{
		stopTimeout: 10 * time.Second,
		max:         10,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &Goroutine{
		ctx:     ctx,
		cancel:  cancel,
		task:    make(chan func(), o.max),
		options: o,
	}
}

// _go: 封装goroutine 使其安全执行
func (g *Goroutine) _go() {
	atomic.AddInt64(&g.n, 1)
	g.wait.Add(1)
	go func() {
		// recover 避免野生goroutine panic后主程退出
		defer func() {
			if err := recover(); err != nil {
				// recover panic
				if g.logger != nil {
					g.logger.Print("Panic", err)
				}
				return
			}
		}()
		defer atomic.AddInt64(&g.n, -1)
		defer g.wait.Done()
		for {
			select {
			case f, ok := <-g.task:
				// channel已经被关闭
				if !ok {
					return
				}
				// 执行函数
				f()
				if atomic.LoadInt64(&g.max) < atomic.LoadInt64(&g.n) {
					// 如果已经超出预定值,则该goroutine退出
					return
				}
			case <-g.ctx.Done():
				// 触发ctx退出
				return
			}
		}
	}()
}

// AddTask: 添加任务
// 直到添加成功为止
func (g *Goroutine) AddTask(f func()) bool {
	// 判断channel是否关闭
	if atomic.LoadInt32(&g.close) != 0 {
		return false
	}
	if atomic.LoadInt64(&g.max) > atomic.LoadInt64(&g.n) {
		g._go()
	}
	g.task <- f
	return true
}

// AddTask: 添加任务
func (g *Goroutine) AddTaskN(ctx context.Context, f func()) bool {
	// 判断channel是否关闭
	if atomic.LoadInt32(&g.close) != 0 {
		return false
	}
	if atomic.LoadInt64(&g.max) > atomic.LoadInt64(&g.n) {
		g._go()
	}
	select {
	case <-ctx.Done():
		return false
	case g.task <- f:
		return true
	}
}

// ChangeMax: 修改pool上限值
func (g *Goroutine) ChangeMax(m int64) {
	atomic.StoreInt64(&g.max, m)
}

// Shutdown: 优雅关闭
// 符合幂等性
func (g *Goroutine) Shutdown() error {
	if atomic.SwapInt32(&g.close, 1) == 1 {
		return ErrRepeatClose
	}
	g.cancel()
	close(g.task)
	err := Delegate(context.TODO(), g.stopTimeout, func(context.Context) error {
		g.wait.Wait()
		return nil
	})
	if g.logger != nil {
		g.logger.Print(err)
	}
	return err
}

// trick: Debug使用
func (g *Goroutine) trick() {
	if g.logger != nil {
		g.logger.Print(atomic.LoadInt64(&g.max), atomic.LoadInt64(&g.n), len(g.task))
	}
}

// Delegate: 委托执行 一般用于回收函数超时控制
func Delegate(c context.Context, t time.Duration, f func(ctx context.Context) error) error {
	ch := make(chan error, 1)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// panic兜底
				switch e := err.(type) {
				case string:
					ch <- errors.New(e)
				case error:
					ch <- e
				default:
					ch <- errors.New(fmt.Sprintf("%+v\n", err))
				}
				return
			}
		}()
		ch <- f(c)
	}()
	// 增加优雅退出超时控制
	var (
		cancel context.CancelFunc
	)
	if t > 0 {
		_, c, cancel = timeout.Shrink(c, t)
	} else {
		c, cancel = context.WithCancel(c)
	}
	defer cancel()
	select {
	case <-c.Done():
		return c.Err()
	case err := <-ch:
		return err
	}
}
