package goroutine

import (
	"Songzhibin/GKit/log"
	"context"
	"sync/atomic"
)

// Goroutine:
type Goroutine struct {
	close int32
	// m: 最大goroutine的数量
	m int64
	// n: 当前goroutine的数量
	n int64
	// 日志库
	log.Logger
	// ctx context
	ctx context.Context
	// cancel
	cancel context.CancelFunc
	// task
	task chan func()
}

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

// Shutdown: 关闭
// 符合幂等性
func (g *Goroutine) Shutdown() {
	if atomic.SwapInt32(&g.close, 1) == 1 {
		return
	}
	g.cancel()
	close(g.task)
}

func (g *Goroutine) trick() {
	g.Logger.Print(atomic.LoadInt64(&g.m), atomic.LoadInt64(&g.n), len(g.task))
}
