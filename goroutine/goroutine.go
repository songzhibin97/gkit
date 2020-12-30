package goroutine

// package goroutine: 管理goroutine并发量托管任务以及兜底

import (
	"Songzhibin/GKit/log"
	"Songzhibin/GKit/timeout"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Goroutine:
type Goroutine struct {
	close int32
	// m: 最大goroutine的数量
	m int64
	// n: 当前goroutine的数量
	n    int64
	wait sync.WaitGroup
	// 日志库
	log.Logger
	// ctx context
	ctx context.Context
	// cancel
	cancel context.CancelFunc
	// task
	task chan func()
}

// NewGoroutine: 实例化方法
func NewGoroutine(ctx context.Context, m int64, logger log.Logger) *Goroutine {
	ctx, cancel := context.WithCancel(ctx)
	return &Goroutine{
		m:      m,
		ctx:    ctx,
		cancel: cancel,
		task:   make(chan func(), m),
		Logger: logger,
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
				g.Print("Panic", err)
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
				if atomic.LoadInt64(&g.m) < atomic.LoadInt64(&g.n) {
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
func (g *Goroutine) AddTask(f func()) bool {
	// 判断channel是否关闭
	if atomic.LoadInt32(&g.close) != 0 {
		return false
	}
	if atomic.LoadInt64(&g.m) > atomic.LoadInt64(&g.n) {
		g._go()
	}
	select {
	case g.task <- f:
		return true
	default:
		// todo 任务丢弃?
		g.Logger.Print("任务丢弃")
		return false
	}
}

// ChangeMax: 修改pool上限值
func (g *Goroutine) ChangeMax(m int64) {
	atomic.StoreInt64(&g.m, m)
}

// Shutdown: 优雅关闭
// 符合幂等性
func (g *Goroutine) Shutdown() {
	if atomic.SwapInt32(&g.close, 1) == 1 {
		return
	}
	g.cancel()
	close(g.task)
	fmt.Println(Delegate(context.TODO(), 10*time.Second, func(context.Context) error {
		g.wait.Wait()
		return nil
	}))
}

// trick: Debug使用
func (g *Goroutine) trick() {
	g.Logger.Print(atomic.LoadInt64(&g.m), atomic.LoadInt64(&g.n), len(g.task))
}

// Delegate: 委托执行 一般用于回收函数超时控制
func Delegate(c context.Context, t time.Duration, f func(ctx context.Context) error) error {
	ch := make(chan error)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				<-ch
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
